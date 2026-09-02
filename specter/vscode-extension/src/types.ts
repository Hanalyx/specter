// @spec spec-vscode

// ---------------------------------------------------------------------------
// Spec index — built by loading all specs into memory from specter output
// ---------------------------------------------------------------------------

export interface ACEntry {
  id: string;
  description: string;
  /** v0.7.0+ optional AC metadata. Present only when the source data carried them. */
  notes?: string;
  approvalGate?: boolean;
  approvalDate?: string;
}

export interface ConstraintEntry {
  id: string;
  description: string;
}

export interface SpecEntry {
  id: string;
  title: string;
  tier: number;
  file: string;
  acs: ACEntry[];
  constraints?: ConstraintEntry[];
  /** Maps constraint ID to the list of AC IDs that reference it */
  constraintReferences?: Record<string, string[]>;
  coveragePct: number;
  status: string;
}

export interface SpecIndex {
  specs: Record<string, SpecEntry>;
}

export interface SpecSummary {
  totalSpecs: number;
  passing: number;
  failing: number;
  fullyCovered: number;
  partiallyCovered: number;
  uncovered: number;
}

// ---------------------------------------------------------------------------
// Coverage report — from specter coverage --json
// ---------------------------------------------------------------------------

export interface SpecCoverageEntry {
  specID: string;
  tier: number;
  totalACs: number;
  coveredACs: string[];
  uncoveredACs: string[];
  coveragePct: number;
  threshold: number;
  passesThreshold: boolean;
  testFiles: string[];
  /**
   * v0.9.0+: path to the .spec.yaml that declared this spec. Emitted by the
   * CLI (may be relative to the workspace root). Used to wire up click-to-
   * open from the Coverage sidebar's spec nodes.
   */
  specFile?: string;
}

/**
 * What every spec-coverage C-44 violation carries, whatever kind it is.
 *
 * A violation must identify the stream it concerns. `stream` is the empty
 * string only for `empty_stream_name`, where there is no name to print and the
 * array position identifies the row instead.
 */
export interface StreamViolation {
  stream: string;
  message: string;
}

/** An entry names a stream the block does not declare. C-44(a). */
export interface UndeclaredStreamError extends StreamViolation {
  kind: 'undeclared_stream';
  resultIndex: number;
}

/** A non-empty stream name appears twice in the block. C-44(b). */
export interface DuplicateStreamError extends StreamViolation {
  kind: 'duplicate_stream';
  streamIndex: number;
}

/** A declared row carries an empty name. C-44(b). */
export interface EmptyStreamNameError extends StreamViolation {
  kind: 'empty_stream_name';
  streamIndex: number;
}

/** A declared row carries a count below zero. C-44(d). */
export interface NegativeCountError extends StreamViolation {
  kind: 'negative_count';
  streamIndex: number;
}

/** A row declares fewer extracted than the entries carrying its label. C-44(e). */
export interface ExtractedBelowEntriesError extends StreamViolation {
  kind: 'extracted_below_entries';
  streamIndex: number;
}

/**
 * One streams-block violation, discriminated on `kind`.
 *
 * Each branch permits exactly one coordinate. `undeclared_stream` is about an
 * entry and carries `resultIndex`; every other kind is about a declared row and
 * carries `streamIndex`. The union is what makes the wrong one a compile error
 * rather than an undefined a renderer has to guard.
 */
export type StreamValidationError =
  | UndeclaredStreamError
  | DuplicateStreamError
  | EmptyStreamNameError
  | NegativeCountError
  | ExtractedBelowEntriesError;

/**
 * Why a coverage run stopped before it measured anything, spec-coverage C-10.
 *
 * The four kinds are declared here and nowhere else. A second union, enum or
 * named constant collection is a copy of policy that cannot know when this one
 * changes, which spec-vscode C-33 forbids.
 */
export interface CoverageStopReason {
  /** What stopped the run. A consumer branches on this. */
  kind: 'manifest_error' | 'invalid_flag' | 'unknown_scope' | 'unmet_precondition';
  /** What a person reads. Required: a kind that explains nothing is not a reason. */
  message: string;
}

export interface CoverageReport {
  entries: SpecCoverageEntry[];
  summary: SpecSummary;
  /**
   * Present only when the run refused, spec-coverage C-10.
   *
   * `entries` and `summary` are still emitted on a refusal, as an empty list
   * and a zeroed summary, and those are uncomputed placeholders rather than
   * measurements. This field is the only thing that separates them from a
   * clean empty workspace, so spec-vscode C-33 requires it to be read before
   * either of them is interpreted.
   */
  stopReason?: CoverageStopReason;
  /**
   * v0.9.0+: per-file parse errors surfaced by `specter coverage --json`.
   * Every report that came through the client carries this field, as [] when
   * nothing failed to parse (C-31). It is optional because the store helpers
   * also accept reports assembled by hand. Used to distinguish the three
   * sidebar states: not-run (report === null), parse-failed (entries empty AND
   * parseErrors non-empty), nothing-to-show (entries empty AND parseErrors
   * empty).
   */
  parseErrors?: CoverageParseError[];
  /**
   * spec-coverage C-44 violations from the results file's `streams` block.
   * The CLI omits the key when the block is absent or consistent, and
   * `SpecterClient` normalizes that absence to `[]`, so a consumer reading a
   * report the client returned can iterate it without a guard (C-31).
   *
   * Optional for the same reason `parseErrors` is: `merged()` reassembles a
   * multi-root report field by field and this one has no coherent multi-root
   * value. Each violation's coordinate points into one results file, so
   * concatenating two roots would hand a reader indexes into a file the
   * element does not name. C-31 binds what a `SpecterClient` method returns,
   * which is where the guarantee is real.
   */
  resultsValidationErrors?: StreamValidationError[];
  /**
   * v0.9.0+: number of .spec.yaml files discovered on disk. When > 0 with
   * entries empty, the workspace has specs that didn't parse — tell the
   * user that, don't suggest `specter init`.
   */
  specCandidatesCount?: number;
  /**
   * v0.9.0+: grouped summary of parseErrors — each entry is one (type, path)
   * that appears in many specs. Sorted by count desc. Enables surfacing
   * schema drift in one sentence ("20 specs: missing `objective`") instead
   * of 20 individual diagnostics.
   */
  parseErrorPatterns?: CoverageParseErrorPattern[];
}

/**
 * The multi-root aggregate, spec-vscode C-33.
 *
 * A distinct type, not CoverageReport with an extra field and not an
 * `extends` of it. A single-folder report and an aggregate answer different
 * questions, and one type wearing both makes every consumer decide which it
 * holds.
 *
 * It deliberately does NOT carry `stopReason`. A lone reason on an aggregate
 * has no owner, and concatenating several loses which folder each belongs to,
 * which is the one thing an operator needs in order to fix one.
 */
export interface MergedCoverageReport {
  entries: SpecCoverageEntry[];
  summary: SpecSummary;
  parseErrors?: CoverageParseError[];
  specCandidatesCount?: number;
  parseErrorPatterns?: CoverageParseErrorPattern[];
  resultsValidationErrors?: CoverageReport['resultsValidationErrors'];
  /**
   * Refusals keyed by the store's own folder key, the value createClientKey
   * produces. The key is the caller's existing handle on a folder, not a
   * display name.
   */
  folderStopReasons?: Record<string, CoverageStopReason>;
}

export interface CoverageParseErrorPattern {
  type: string;
  path?: string;
  count: number;
  exampleFile?: string;
  files?: string[];
}

export interface CoverageParseError {
  file: string;
  path?: string;
  type?: string;
  message: string;
  line?: number;
  column?: number;
}

// ---------------------------------------------------------------------------
// Diagnostics — from specter parse --json and specter check --json
// ---------------------------------------------------------------------------

/**
 * One entry of the `errors` array `specter parse --json` emits. The names are
 * the CLI's own (C-32): it emits `path`, `type`, and `message` on every error,
 * and `line` only on a YAML syntax error. It emits no column, because no code
 * path in the parser assigns that field, and it puts the file name on the
 * document rather than on each error inside it.
 *
 * Until v1.9.0 this declared `col` and `code`, which the CLI names `column`
 * and `type`, and declared `file` and `line` as required. Both reads yielded
 * `undefined`, and the arithmetic on them yielded `NaN` (SP-SP-025).
 */
export interface SpecterParseError {
  path: string;
  type: string;
  message: string;
  /** Present on a YAML syntax error only. 1-indexed. */
  line?: number;
}

/**
 * One entry of the `diagnostics` array `specter check --json` emits, after the
 * client converts the document's keys to the camelCase shape the extension
 * reads (C-32). The CLI emits `kind`, `severity`, `message`, and `spec_id` on
 * every diagnostic, and the three optional fields only when non-empty. There
 * were four until spec-check 2.0.0 retracted version-change classification and
 * `changeType` went with it (bugs/done/SP-SP-018).
 *
 * A check diagnostic carries no position of any kind and names no file, so
 * neither is declared. Until v1.9.0 both were declared as required, alongside
 * a `specID` the unconverted document never carried (SP-SP-025).
 */
export interface SpecterCheckDiagnostic {
  kind: string;
  severity: 'error' | 'warning' | 'info';
  specID: string;
  message: string;
  constraintID?: string;
  constraintType?: string;
  // No `changeType`. spec-check 2.0.0 retracted version-change classification,
  // so no check diagnostic carries `change_type`, and C-32 forbids declaring a
  // member no document carries (bugs/SP-SP-018).
  details?: string;
}

// ---------------------------------------------------------------------------
// Diagnostic wrapper (VS-Code-agnostic)
// ---------------------------------------------------------------------------

export interface ExtensionDiagnostic {
  severity: 'error' | 'warning' | 'info';
  source: string;
  message: string;
  range: {
    start: { line: number; character: number };
    end: { line: number; character: number };
  };
}

// ---------------------------------------------------------------------------
// Completion items (VS-Code-agnostic)
// ---------------------------------------------------------------------------

export interface CompletionItem {
  label: string;
  insertText: string;
  detail?: string;
  documentation?: string;
  sortText?: string;
}

// ---------------------------------------------------------------------------
// Hover result (VS-Code-agnostic)
// ---------------------------------------------------------------------------

export interface HoverResult {
  contents: string;
}

// ---------------------------------------------------------------------------
// Quick-fix result
// ---------------------------------------------------------------------------

export interface QuickFixResult {
  insertLine: number;
  text: string;
  isSnippet: boolean;
}

// ---------------------------------------------------------------------------
// Decoration (VS-Code-agnostic)
// ---------------------------------------------------------------------------

// No 'gap'. spec-vscode 5.0.0 retracted the gray-dash rendering, and a kind
// nothing can produce is a declaration that reads as a guarantee (SP-SP-019).
export type DecorationKind = 'covered' | 'uncovered';

export interface ACDecoration {
  acID: string;
  kind: DecorationKind;
  endOfLineText?: string;
}

// ---------------------------------------------------------------------------
// Tree view nodes (VS-Code-agnostic)
// ---------------------------------------------------------------------------

export interface TestFileNode {
  path: string;
}

export interface ACNode {
  id: string;
  icon: DecorationKind;
  children: TestFileNode[];
}

export interface SpecTreeNode {
  specID: string;
  file: string;
  children: ACNode[];
}

/**
 * v0.8.0+: synthetic node shown in the Coverage sidebar when there is no
 * coverage data to render (report null, or report with zero entries). Gives
 * the user a visible state + action instead of a silently empty panel.
 */
export interface TreeMessageNode {
  kind: 'message';
  label: string;
  detail?: string;
  iconId?: string; // optional theme icon id (e.g. 'info', 'warning')
}

/**
 * Tagged variant of SpecTreeNode so buildCoverageTreeRoot can return either
 * real spec nodes or a message node without the caller unpacking by index.
 */
export interface SpecTreeRootSpec {
  kind: 'spec';
  specID: string;
  file: string;
  children: ACNode[];
}

/**
 * v0.9.0+: collapsible "Failed to parse" group that appears alongside
 * passing spec nodes when the coverage report contains parseErrors. Each
 * child is a clickable file the user can open to fix the error. This
 * replaces the v0.8.x all-or-nothing behavior where parse failures hid
 * every passing spec.
 */
export interface ParseErrorGroupNode {
  kind: 'parseErrorGroup';
  label: string;
  children: ParseErrorFileNode[];
}

export interface ParseErrorFileNode {
  kind: 'parseErrorFile';
  file: string;
  message: string;
  line?: number;
}

export type SpecTreeRootNode =
  | SpecTreeRootSpec
  | TreeMessageNode
  | ParseErrorGroupNode;

// ---------------------------------------------------------------------------
// File decoration (VS-Code-agnostic)
// ---------------------------------------------------------------------------

export interface FileDecoration {
  badge: string;
  color: string;
  tooltip?: string;
}

// ---------------------------------------------------------------------------
// AC suggestion (tf-idf)
// ---------------------------------------------------------------------------

export interface ACSuggestion {
  specID: string;
  acID: string;
  description: string;
  score: number;
}

// ---------------------------------------------------------------------------
// Definition target (VS-Code-agnostic)
// ---------------------------------------------------------------------------

export interface DefinitionTarget {
  file: string;
  line: number;
}

// ---------------------------------------------------------------------------
// Insight card
// ---------------------------------------------------------------------------

export interface InsightCard {
  specID: string;
  summary: string;
  uncoveredACDetails: Array<{ id: string; description: string }>;
  constraintCallouts: Array<{ constraintID: string; description: string }>;
}

// ---------------------------------------------------------------------------
// Drift
// ---------------------------------------------------------------------------

export interface DriftBaseline {
  specID: string;
  acID: string;
  specFileHashAtAnnotation: string;
}

export interface DriftResult {
  hasDrift: boolean;
  changeClass: string | null;
}

// ---------------------------------------------------------------------------
// Notification
// ---------------------------------------------------------------------------

export interface NotificationResult {
  kind: 'none' | 'status-bar-only' | 'information-toast' | 'warning-toast' | 'modal-error';
  actions?: string[];
}
