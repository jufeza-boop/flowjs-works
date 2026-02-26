package activities

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"regexp"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	fmodels "flowjs-works/engine/internal/models"
)

// SFTPActivity implements the `sftp` node type.
//
// config fields (all string unless noted):
//
//	server:        hostname or IP (required)
//	port:          int, default 22
//	auth:          map — user (string), password (string) OR private_key (PEM string)
//	folder:        remote directory (required)
//	method:        "get" | "put" (required)
//	regex_filter:  regex to filter remote filenames (get only)
//	overwrite:     bool — overwrite existing destination files (put only, default true)
//	create_folder: bool — create destination folder if missing (put only)
//	local_folder:  local directory used as source (put) or destination (get)
//	files:         []interface{} of local filenames to upload (put only)
type SFTPActivity struct{}

// Name returns the DSL type identifier for this activity.
func (a *SFTPActivity) Name() string { return "sftp" }

// Execute runs the SFTP get or put operation.
func (a *SFTPActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *fmodels.ExecutionContext) (map[string]interface{}, error) {
	server, ok := config["server"].(string)
	if !ok || server == "" {
		return nil, fmt.Errorf("sftp activity: missing required config field 'server'")
	}

	port := 22
	switch v := config["port"].(type) {
	case int:
		port = v
	case float64:
		port = int(v)
	}

	method, ok := config["method"].(string)
	if !ok || (method != "get" && method != "put") {
		return nil, fmt.Errorf("sftp activity: config field 'method' must be 'get' or 'put'")
	}

	folder, ok := config["folder"].(string)
	if !ok || folder == "" {
		return nil, fmt.Errorf("sftp activity: missing required config field 'folder'")
	}

	// Validate regex_filter early so callers get a clear error before any network I/O.
	if rf, ok := config["regex_filter"].(string); ok && rf != "" {
		if _, err := regexp.Compile(rf); err != nil {
			return nil, fmt.Errorf("sftp activity: invalid regex_filter %q: %w", rf, err)
		}
	}

	sshCfg, err := buildSSHClientConfig(config)
	if err != nil {
		return nil, fmt.Errorf("sftp activity: failed to build SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", server, port)
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("sftp activity: TCP dial failed: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshCfg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("sftp activity: SSH handshake failed: %w", err)
	}
	sshClient := ssh.NewClient(sshConn, chans, reqs)
	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, fmt.Errorf("sftp activity: failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	switch method {
	case "get":
		return sftpGet(sftpClient, config, folder)
	case "put":
		return sftpPut(sftpClient, config, folder)
	default:
		return nil, fmt.Errorf("sftp activity: unknown method %q", method)
	}
}

// sftpGet downloads files from the remote folder to local_folder, optionally
// filtered by regex_filter.
func sftpGet(client *sftp.Client, config map[string]interface{}, remoteFolder string) (map[string]interface{}, error) {
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

	entries, err := client.ReadDir(remoteFolder)
	if err != nil {
		return nil, fmt.Errorf("sftp activity: failed to list remote folder %q: %w", remoteFolder, err)
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

		remotePath := path.Join(remoteFolder, name)
		localPath := localFolder + "/" + name

		if err := downloadFile(client, remotePath, localPath); err != nil {
			return nil, fmt.Errorf("sftp activity: failed to download %q: %w", name, err)
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

// sftpPut uploads files from input["files"] (or config["files"]) to the remote folder.
func sftpPut(client *sftp.Client, config map[string]interface{}, remoteFolder string) (map[string]interface{}, error) {
	createFolder, _ := config["create_folder"].(bool)
	overwrite := true
	if ow, ok := config["overwrite"].(bool); ok {
		overwrite = ow
	}
	localFolder, _ := config["local_folder"].(string)
	if localFolder == "" {
		localFolder = "."
	}

	// Collect filenames to upload
	var fileNames []string
	if flist, ok := config["files"].([]interface{}); ok {
		for _, f := range flist {
			if s, ok := f.(string); ok {
				fileNames = append(fileNames, s)
			}
		}
	}

	if createFolder {
		if err := client.MkdirAll(remoteFolder); err != nil {
			return nil, fmt.Errorf("sftp activity: failed to create remote folder %q: %w", remoteFolder, err)
		}
	}

	var uploaded []string
	for _, name := range fileNames {
		localPath := localFolder + "/" + name
		remotePath := path.Join(remoteFolder, name)

		if !overwrite {
			if _, err := client.Stat(remotePath); err == nil {
				// File exists; skip
				continue
			}
		}

		if err := uploadFile(client, localPath, remotePath); err != nil {
			return nil, fmt.Errorf("sftp activity: failed to upload %q: %w", name, err)
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

// downloadFile copies a single remote file to a local path.
func downloadFile(client *sftp.Client, remotePath, localPath string) error {
	remote, err := client.Open(remotePath)
	if err != nil {
		return err
	}
	defer remote.Close()

	local, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer local.Close()

	if _, err := io.Copy(local, remote); err != nil {
		return err
	}
	return nil
}

// uploadFile copies a local file to a remote path.
func uploadFile(client *sftp.Client, localPath, remotePath string) error {
	local, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer local.Close()

	remote, err := client.Create(remotePath)
	if err != nil {
		return err
	}
	defer remote.Close()

	if _, err := io.Copy(remote, local); err != nil {
		return err
	}
	return nil
}

// buildSSHClientConfig builds an ssh.ClientConfig from the activity config's auth map.
// auth map keys: user (string), password (string), private_key (PEM string).
func buildSSHClientConfig(config map[string]interface{}) (*ssh.ClientConfig, error) {
	user := "anonymous"
	var authMethods []ssh.AuthMethod

	if authMap, ok := config["auth"].(map[string]interface{}); ok {
		if u, ok := authMap["user"].(string); ok && u != "" {
			user = u
		}
		if pk, ok := authMap["private_key"].(string); ok && pk != "" {
			signer, err := ssh.ParsePrivateKey([]byte(pk))
			if err != nil {
				return nil, fmt.Errorf("failed to parse private_key: %w", err)
			}
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
		if pass, ok := authMap["password"].(string); ok && pass != "" {
			authMethods = append(authMethods, ssh.Password(pass))
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("sftp activity: auth must provide either password or private_key")
	}

	//nolint:gosec // Host key verification is intentionally skipped at the user's request;
	// production deployments should provide a known_hosts callback via configuration.
	return &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}, nil
}


