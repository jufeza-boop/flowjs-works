package activities

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/hirochachacha/go-smb2"

	fmodels "flowjs-works/engine/internal/models"
)

// SMBActivity implements the `smb` node type (SMB2/3 protocol).
//
// config fields:
//
//	server:        hostname or IP (required)
//	port:          int, default 445
//	share:         SMB share name, e.g. "shared" (required)
//	auth:          map — user (string), password (string), domain (string, optional)
//	folder:        directory path inside the share (default "/")
//	method:        "get" | "put" (required)
//	regex_filter:  regex to filter filenames (get only)
//	overwrite:     bool — overwrite existing destination files (put only, default true)
//	local_folder:  local directory used as source (put) or destination (get)
//	files:         []interface{} of filenames to upload (put only)
type SMBActivity struct{}

// Name returns the DSL type identifier for this activity.
func (a *SMBActivity) Name() string { return "smb" }

// Execute runs the SMB get or put operation.
func (a *SMBActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *fmodels.ExecutionContext) (map[string]interface{}, error) {
	server, ok := config["server"].(string)
	if !ok || server == "" {
		return nil, fmt.Errorf("smb activity: missing required config field 'server'")
	}

	port := 445
	switch v := config["port"].(type) {
	case int:
		port = v
	case float64:
		port = int(v)
	}

	share, ok := config["share"].(string)
	if !ok || share == "" {
		return nil, fmt.Errorf("smb activity: missing required config field 'share'")
	}

	method, ok := config["method"].(string)
	if !ok || (method != "get" && method != "put") {
		return nil, fmt.Errorf("smb activity: config field 'method' must be 'get' or 'put'")
	}

	folder, _ := config["folder"].(string)
	if folder == "" {
		folder = "."
	}

	// Validate regex_filter early so callers get a clear error before any network I/O.
	if rf, ok := config["regex_filter"].(string); ok && rf != "" {
		if _, err := regexp.Compile(rf); err != nil {
			return nil, fmt.Errorf("smb activity: invalid regex_filter %q: %w", rf, err)
		}
	}

	// Extract auth
	user, password, domain := extractSMBAuth(config)

	addr := fmt.Sprintf("%s:%d", server, port)
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("smb activity: TCP dial failed: %w", err)
	}
	defer conn.Close()

	dialer := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     user,
			Password: password,
			Domain:   domain,
		},
	}

	session, err := dialer.Dial(conn)
	if err != nil {
		return nil, fmt.Errorf("smb activity: SMB2 session failed: %w", err)
	}
	defer session.Logoff()

	fs, err := session.Mount(share)
	if err != nil {
		return nil, fmt.Errorf("smb activity: failed to mount share %q: %w", share, err)
	}
	defer fs.Umount()

	switch method {
	case "get":
		return smbGet(fs, config, folder)
	case "put":
		return smbPut(fs, config, folder)
	default:
		return nil, fmt.Errorf("smb activity: unknown method %q", method)
	}
}

// smbGet downloads files from the SMB share/folder to local_folder.
func smbGet(fs *smb2.Share, config map[string]interface{}, remoteFolder string) (map[string]interface{}, error) {
	localFolder, _ := config["local_folder"].(string)
	if localFolder == "" {
		localFolder = "."
	}

	// regex_filter was already validated in Execute; compile here to apply it.
	var filter *regexp.Regexp
	if rf, ok := config["regex_filter"].(string); ok && rf != "" {
		// Error is ignored — compilation was already validated in Execute.
		filter, _ = regexp.Compile(rf)
	}

	entries, err := fs.ReadDir(remoteFolder)
	if err != nil {
		return nil, fmt.Errorf("smb activity: failed to list remote folder %q: %w", remoteFolder, err)
	}

	var downloaded []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filter != nil && !filter.MatchString(name) {
			continue
		}

		remotePath := filepath.Join(remoteFolder, name)
		localPath := filepath.Join(localFolder, name)

		if err := smbDownloadFile(fs, remotePath, localPath); err != nil {
			return nil, fmt.Errorf("smb activity: failed to download %q: %w", name, err)
		}
		downloaded = append(downloaded, name)
	}

	if downloaded == nil {
		downloaded = []string{}
	}
	return map[string]interface{}{
		"files_downloaded": downloaded,
		"count":            len(downloaded),
	}, nil
}

// smbPut uploads files from config["files"] (or input["files"]) to the SMB share/folder.
func smbPut(fs *smb2.Share, config map[string]interface{}, remoteFolder string) (map[string]interface{}, error) {
	localFolder, _ := config["local_folder"].(string)
	if localFolder == "" {
		localFolder = "."
	}

	overwrite := true
	if ow, ok := config["overwrite"].(bool); ok {
		overwrite = ow
	}

	var fileNames []string
	if flist, ok := config["files"].([]interface{}); ok {
		for _, f := range flist {
			if s, ok := f.(string); ok {
				fileNames = append(fileNames, s)
			}
		}
	}

	var uploaded []string
	for _, name := range fileNames {
		remotePath := filepath.Join(remoteFolder, name)
		localPath := filepath.Join(localFolder, name)

		if !overwrite {
			if _, err := fs.Stat(remotePath); err == nil {
				continue
			}
		}

		if err := smbUploadFile(fs, localPath, remotePath); err != nil {
			return nil, fmt.Errorf("smb activity: failed to upload %q: %w", name, err)
		}
		uploaded = append(uploaded, name)
	}

	if uploaded == nil {
		uploaded = []string{}
	}
	return map[string]interface{}{
		"files_uploaded": uploaded,
		"count":          len(uploaded),
	}, nil
}

// smbDownloadFile copies a single file from the SMB share to a local path.
func smbDownloadFile(fs *smb2.Share, remotePath, localPath string) error {
	remote, err := fs.Open(remotePath)
	if err != nil {
		return err
	}
	defer remote.Close()

	local, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer local.Close()

	_, err = io.Copy(local, remote)
	return err
}

// smbUploadFile copies a local file to the SMB share.
func smbUploadFile(fs *smb2.Share, localPath, remotePath string) error {
	local, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer local.Close()

	remote, err := fs.Create(remotePath)
	if err != nil {
		return err
	}
	defer remote.Close()

	_, err = io.Copy(remote, local)
	return err
}

// extractSMBAuth reads user / password / domain from config["auth"].
func extractSMBAuth(config map[string]interface{}) (user, password, domain string) {
	if authMap, ok := config["auth"].(map[string]interface{}); ok {
		user, _ = authMap["user"].(string)
		password, _ = authMap["password"].(string)
		domain, _ = authMap["domain"].(string)
	}
	return user, password, domain
}
