# Technical Risk Analysis — flowjs-works

> Top 3 architectural risks and their proposed mitigations.

---

## Risk 1: Dynamic Data Mapping Between Nodes (JSONPath Resolution)

### Description

Nodes reference outputs of previous nodes via JSONPath expressions (e.g. `$.nodes.db_insert_user.output.id`). This creates a runtime dependency graph where:

- Referenced nodes may not have executed yet (parallel branches).
- Referenced paths may not exist (typos, schema changes, optional fields).
- Deeply nested paths with array indexing add parsing complexity.

A single resolution failure can silently corrupt the entire flow or cause cryptic runtime errors.

### Impact

**High** — This is the backbone of inter-node communication. Every `input_mapping` in every node depends on it.

### Proposed Solution

1. **Static validation at design time**: In the Designer UI, validate all JSONPath references against the known output schemas of upstream nodes. Flag unresolvable references as warnings before deployment.

2. **Strict runtime resolver with typed errors**: The Go `ExecutionContext.Resolve()` function should return a `(value, error)` tuple with descriptive errors like `"node 'db_insert_user' has not been executed yet"` or `"path '$.nodes.X.output.email' not found in output"`.

3. **Fallback values**: Allow an optional `default` field in `input_mapping` entries so nodes can provide fallback values for optional references:
   ```json
   "input_mapping": {
     "email": { "path": "$.nodes.normalize.output.email", "default": "" }
   }
   ```

4. **Execution order validation**: At process load time, topologically sort nodes based on transitions and verify that all JSONPath references point to nodes that will execute *before* the referencing node.

---

## Risk 2: Security in JavaScript Code Execution (Goja Sandbox)

### Description

The `code` node type allows users to write arbitrary JavaScript that runs inside the Go engine via Goja. This introduces risks of:

- **Infinite loops** or CPU-intensive operations that freeze the engine.
- **Memory exhaustion** from unbounded allocations.
- **File system / network access** if the sandbox is not properly isolated.
- **Prototype pollution** or other JS-level attacks that escape the sandbox context.

### Impact

**Critical** — A malicious or buggy script could crash the entire flow runner, affecting all flows on the same instance.

### Proposed Solution

1. **Execution timeout**: Wrap every Goja execution in a goroutine with a `context.WithTimeout`. If the script exceeds the configured `timeout` (default: 5s), cancel the VM via Goja's `Interrupt()` method.

2. **Memory limits**: Set a max heap size for the Goja runtime. Monitor allocations and interrupt if the limit is exceeded. Consider running scripts in a separate process with OS-level `ulimit` controls for production.

3. **API surface whitelisting**: Only expose a minimal set of globals to the JS runtime:
   - `input` — the resolved input mapping data (read-only).
   - `console.log` — routed to the audit logger.
   - Pure utility functions (`JSON.parse`, `JSON.stringify`, `Array.map`, etc.).
   - **Block**: `require`, `import`, `fetch`, `fs`, `process`, `eval`, `Function`.

4. **Immutable input**: Pass the input object as a deep-frozen copy so scripts cannot mutate shared state.

5. **Output validation**: The script must return a JSON-serializable value. Validate the output before storing it in the execution context. Reject functions, circular references, and symbols.

---

## Risk 3: UI Graph ↔ JSON DSL Serialization Fidelity

### Description

The Designer UI uses React Flow's internal graph model (nodes with `position`, `data`, `type`; edges with `source`, `target`, `sourceHandle`, `targetHandle`). This must be losslessly serialized to/from the JSON DSL (which has no UI-specific fields) and vice-versa. Risks include:

- **Data loss on round-trip**: UI-specific metadata (positions, collapsed state, zoom level) might be lost when saving to DSL and reloading.
- **Divergent models**: The React Flow graph model and the DSL may evolve independently, causing deserialization failures.
- **Transition semantics mapping**: React Flow edges are generic; the DSL has typed transitions (`success`, `error`, `condition`, `nocondition`) that need to be encoded in edge metadata.

### Impact

**Medium-High** — If the serializer is not robust, users will lose their canvas layout, edges will become mistyped, or deployed flows will differ from what was designed.

### Proposed Solution

1. **Separation of concerns — two-layer storage**:
   - **DSL layer** (`FlowDSL`): The runtime contract. Contains only execution-relevant data.
   - **Layout layer** (separate JSON): Contains `{ nodeId: { x, y }, zoom, viewport }` metadata. Stored alongside the DSL but never sent to the engine.
   
   This allows the DSL to remain clean while preserving the visual state.

2. **Canonical serializer with round-trip tests**: Maintain a `serializer.ts` module with:
   - `graphToDSL(nodes: DesignerNode[], edges: DesignerEdge[]): FlowDSL`
   - `dslToGraph(dsl: FlowDSL, layout?: LayoutMap): { nodes: DesignerNode[], edges: DesignerEdge[] }`
   
   Write **snapshot tests** that verify `dslToGraph(graphToDSL(graph)) ≈ graph` for every node and transition type.

3. **Edge metadata encoding**: Store the transition `type` and `condition` in the React Flow edge's `data` property:
   ```typescript
   const edge: DesignerEdge = {
     id: `${from}-${to}`,
     source: from,
     target: to,
     data: { transitionType: 'condition', condition: '$.nodes.X.status == "success"' }
   }
   ```

4. **Schema versioning**: Include a `dsl_version` field in the DSL (already present as `definition.version`). The serializer should handle migration from older schema versions to the current one.
