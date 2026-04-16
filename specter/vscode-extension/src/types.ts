// @spec spec-vscode

// ---------------------------------------------------------------------------
// Spec index — built by loading all specs into memory from specter output
// ---------------------------------------------------------------------------

export interface ACEntry {
  id: string;
  description: string;
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
}

export interface CoverageReport {
  entries: SpecCoverageEntry[];
  summary: SpecSummary;
}

// ---------------------------------------------------------------------------
// Diagnostics — from specter parse --json and specter check --json
// ---------------------------------------------------------------------------

export interface SpecterParseError {
  file: string;
  line: number;
  col: number;
  message: string;
  code: string;
}

export interface SpecterCheckDiagnostic {
  kind: string;
  severity: 'error' | 'warning' | 'info';
  specID: string;
  constraintID?: string;
  message: string;
  file: string;
  line: number;
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

export type DecorationKind = 'covered' | 'uncovered' | 'gap';

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
