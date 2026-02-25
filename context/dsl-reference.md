# DSL Quick Reference — flowjs-works

> Canonical types live in `apps/designer/src/types/dsl.ts` (TypeScript) and `services/engine/internal/models/process.go` (Go).

## Top-Level Structure

```json
{
  "definition": { ... },
  "trigger":    { ... },
  "nodes":      [ ... ],
  "transitions": [ ... ]
}
```

## Trigger Types

| Type | `trigger.type` | Key Config Fields | Output Shape |
|------|---------------|-------------------|--------------|
| Cron | `cron` | `expression` | `datetime` |
| REST | `rest` | `path`, `method`, `schema_validation` | `method`, `headers`, `body`, `auth`, `timeout` |
| SOAP | `soap` | `path`, `wsdl` | `method`, `headers`, `body` |
| RabbitMQ | `rabbitmq` | `url_amqp`, `queue`, `vhost` | `payload`, `properties` (`delivery_mode`, `headers`) |
| MCP | `mcp` | `version`, `capabilities` | `tool_request` (`method`, `params`, `arguments`), `client_context` |
| Manual | `manual` | — | User-provided payload |

## Node Types

| Type | `node.type` | Key Config Fields |
|------|------------|-------------------|
| HTTP | `http` | `url`, `method`, `headers`, `data`, `auth`, `timeout` |
| SFTP | `sftp` | `server`, `port`, `auth`, `folder`, `method` (get/put), `regex_filter`, `overwrite`, `create_folder` |
| S3 | `s3` | `bucket`, `region`, `auth`, `folder`, `method` (get/put) |
| SMB | `smb` | `server`, `share`, `auth`, `folder`, `method` (get/put) |
| Mail | `mail` | `host`, `port`, `security`, `auth`, `action` (send/receive), action-specific fields |
| RabbitMQ | `rabbitmq` | `url_amqp`, `vhost`, `exchange`, `routing_key`, `payload`, `properties` |
| SQL | `sql` | `engine`, `host`, `port`, `database`, `schema`, `credentials`, `query`, `params`, `timeout`, `autocommit`, `ssl_mode` |
| Code | `code` | `script` (JS source) |
| Log | `log` | `level` (ERROR/WARNING/INFO/DEBUG), `message` |
| Transform | `transform` | `transform_type` (json2csv/xml2json/json2xml), `data`, `spec` |
| File | `file` | `operation` (create/delete/read), `path`, `content`, `mode` (overwrite/append) |

## Transition Types

| Type | `transition.type` | Semantics |
|------|-------------------|-----------|
| Success | `success` | Taken when source node succeeds |
| Error | `error` | Taken when source node fails (visual try/catch) |
| Condition | `condition` | Taken when `condition` expression is truthy |
| NoCondition | `nocondition` | Else branch; only valid alongside a `condition` from the same node |

## Secret References

Nodes that need credentials use `secret_ref` instead of inline secrets:

```json
{
  "id": "query_users",
  "type": "sql",
  "secret_ref": "sec_postgres_main",
  "config": {
    "engine": "postgres",
    "query": "SELECT * FROM users WHERE active = $1",
    "params": ["true"]
  }
}
```

## JSONPath Data References

All `input_mapping` values use JSONPath syntax:

| Pattern | Resolves To |
|---------|------------|
| `$.trigger.body` | Trigger output body |
| `$.trigger.headers.date` | Specific trigger header |
| `$.nodes.<id>.output` | Full output of node `<id>` |
| `$.nodes.<id>.output.email` | Specific field from node output |
| `$.nodes.<id>.status` | Execution status of node `<id>` |
