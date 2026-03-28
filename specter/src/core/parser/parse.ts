/**
 * spec-parse: YAML-to-SpecAST parser.
 *
 * Pure function. No CLI deps, no I/O.
 * Validates .spec.yaml content against the canonical JSON Schema
 * and returns a typed SpecAST or structured parse errors.
 *
 * @spec spec-parse
 */

import Ajv2020, { type ErrorObject } from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import { parseDocument, YAMLParseError } from 'yaml';
import specSchema from '../schema/spec-schema.json' with { type: 'json' };
import type { SpecAST, SpecDocument } from '../schema/types.js';
import type { ParseError, ParseResult } from './errors.js';

// C-01: Validate against canonical JSON Schema
// C-07: Collect all errors (allErrors: true)
// C-03: Reject unknown fields (the schema has additionalProperties: false)
const ajv = new Ajv2020({ allErrors: true, strict: false });
addFormats(ajv);
const validate = ajv.compile<SpecDocument>(specSchema);

/**
 * Convert an Ajv error path to a human-readable JSON path.
 * e.g., "/spec/constraints/0/id" -> "spec.constraints[0].id"
 */
function ajvPathToJsonPath(instancePath: string): string {
  return instancePath
    .replace(/^\//, '')
    .replace(/\//g, '.')
    .replace(/\.(\d+)\./g, '[$1].')
    .replace(/\.(\d+)$/, '[$1]');
}

/**
 * Build a ParseError from an Ajv ErrorObject.
 * C-02: Include field path. Line numbers are added in a later pass via YAML source mapping.
 */
function ajvErrorToParseError(error: ErrorObject, yamlLineMap: Map<string, number>): ParseError {
  const keyword = error.keyword;
  let path: string;

  if (keyword === 'required') {
    const missingProp = error.params?.missingProperty as string | undefined;
    const parentPath = ajvPathToJsonPath(error.instancePath);
    path = parentPath ? `${parentPath}.${missingProp}` : (missingProp ?? '');
  } else if (keyword === 'additionalProperties') {
    const extra = error.params?.additionalProperty as string | undefined;
    const parentPath = ajvPathToJsonPath(error.instancePath);
    path = parentPath ? `${parentPath}.${extra}` : (extra ?? '');
  } else {
    path = ajvPathToJsonPath(error.instancePath);
  }

  const line = yamlLineMap.get(path);

  return {
    path,
    type: keyword,
    message: error.message ?? `Validation failed: ${keyword}`,
    ...(line !== undefined ? { line } : {}),
  };
}

/**
 * Build a line number map from a YAML document.
 * Maps JSON paths (e.g., "spec.id") to their YAML source line numbers.
 */
function buildLineMap(yamlContent: string): Map<string, number> {
  const lineMap = new Map<string, number>();

  try {
    const doc = parseDocument(yamlContent);
    if (!doc.contents) return lineMap;

    function walk(node: unknown, pathParts: string[]): void {
      if (!node || typeof node !== 'object') return;

      if ('items' in node && Array.isArray((node as { items: unknown[] }).items)) {
        const mapNode = node as {
          items: Array<{ key?: { value?: string; range?: number[] }; value?: unknown }>;
        };
        for (const pair of mapNode.items) {
          if (pair.key && 'value' in pair.key) {
            const key = String(pair.key.value);
            const currentPath = [...pathParts, key].join('.');
            if (pair.key.range) {
              const line = yamlContent.slice(0, pair.key.range[0]).split('\n').length;
              lineMap.set(currentPath, line);
            }
            if (pair.value && typeof pair.value === 'object') {
              if (
                'items' in pair.value &&
                Array.isArray((pair.value as { items: unknown[] }).items)
              ) {
                const seqItems = (pair.value as { items: unknown[] }).items;
                if (
                  seqItems.length > 0 &&
                  seqItems[0] &&
                  typeof seqItems[0] === 'object' &&
                  'items' in (seqItems[0] as Record<string, unknown>)
                ) {
                  // Array of maps
                  for (let i = 0; i < seqItems.length; i++) {
                    walk(seqItems[i], [...pathParts, key, String(i)]);
                  }
                }
              } else {
                walk(pair.value, [...pathParts, key]);
              }
            }
          }
        }
      }
    }

    walk(doc.contents, []);
  } catch {
    // If line mapping fails, we still return errors — just without line numbers
  }

  return lineMap;
}

/**
 * Parse a YAML string into a validated SpecAST.
 *
 * C-08: Pure function — no I/O, no CLI deps.
 * C-05: Handles YAML syntax errors gracefully.
 * C-06: YAML anchors/aliases resolved by the yaml library.
 * C-01: Validates against canonical JSON Schema.
 * C-07: Collects all validation errors.
 * C-04: Returns typed SpecAST on success.
 *
 * @param yamlContent - Raw YAML string content
 * @returns ParseResult containing either the validated SpecAST or an array of ParseErrors
 */
export function parseSpec(yamlContent: string): ParseResult<SpecAST> {
  // Step 1: Parse YAML (C-05: graceful syntax error handling, C-06: anchors resolved)
  let parsed: unknown;
  try {
    const doc = parseDocument(yamlContent, { merge: true });
    if (doc.errors.length > 0) {
      // C-02: Include line numbers from YAML errors
      const errors: ParseError[] = doc.errors.map((err: YAMLParseError) => ({
        path: '',
        type: 'yaml_syntax',
        message: err.message,
        ...(err.linePos ? { line: err.linePos[0].line, column: err.linePos[0].col } : {}),
      }));
      return { ok: false, errors };
    }
    parsed = doc.toJS();
  } catch (err) {
    // C-05: Never crash — return structured error
    if (err instanceof YAMLParseError) {
      return {
        ok: false,
        errors: [
          {
            path: '',
            type: 'yaml_syntax',
            message: err.message,
            ...(err.linePos ? { line: err.linePos[0].line, column: err.linePos[0].col } : {}),
          },
        ],
      };
    }
    return {
      ok: false,
      errors: [
        {
          path: '',
          type: 'yaml_syntax',
          message: err instanceof Error ? err.message : 'Unknown YAML parse error',
        },
      ],
    };
  }

  // Step 2: Validate against JSON Schema (C-01, C-03, C-07)
  const valid = validate(parsed);
  if (!valid && validate.errors) {
    const lineMap = buildLineMap(yamlContent);
    const errors: ParseError[] = validate.errors.map((err) => ajvErrorToParseError(err, lineMap));
    return { ok: false, errors };
  }

  // Step 3: Return typed SpecAST (C-04)
  const doc = parsed as SpecDocument;
  return { ok: true, value: doc.spec };
}

/**
 * Parse multiple YAML strings. Convenience wrapper over parseSpec.
 *
 * @param entries - Array of [filename, yamlContent] pairs
 * @returns Map of filename to ParseResult
 */
export function parseSpecs(entries: Array<[string, string]>): Map<string, ParseResult<SpecAST>> {
  const results = new Map<string, ParseResult<SpecAST>>();
  for (const [filename, content] of entries) {
    results.set(filename, parseSpec(content));
  }
  return results;
}
