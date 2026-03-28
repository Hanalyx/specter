/**
 * Parse error types for spec-parse.
 *
 * @spec spec-parse
 */

export interface ParseError {
  /** JSON path to the failing field (e.g., "spec.id", "spec.constraints[0].id") */
  path: string;
  /** Error type (e.g., "required", "pattern", "additionalProperties", "yaml_syntax") */
  type: string;
  /** Human-readable error message */
  message: string;
  /** YAML line number where the error occurred, if available */
  line?: number;
  /** YAML column number, if available */
  column?: number;
}

export type ParseResult<T> = { ok: true; value: T } | { ok: false; errors: ParseError[] };
