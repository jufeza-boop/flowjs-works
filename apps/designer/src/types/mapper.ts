/** A field in a JSON schema — can be nested */
export interface SchemaField {
  key: string
  /** JSONPath to reach this field, e.g. $.nodes.my_node.output.email */
  path: string
  type: 'string' | 'number' | 'boolean' | 'object' | 'array' | 'unknown'
  children?: SchemaField[]
}

/** A visual connection from a source field to a target input-mapping key */
export interface MappingConnection {
  /** The source schema field being connected */
  sourceField: SchemaField
  /** The target input-mapping key (left side of input_mapping object) */
  targetKey: string
  /** Optional inline JS transformation script for Function Link style */
  transformScript?: string
}
