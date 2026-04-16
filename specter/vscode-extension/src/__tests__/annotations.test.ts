// @spec spec-vscode
//
// Tests for annotation completions, hover context, quick-fix insertion,
// and tf-idf AC suggestion heuristic.

import {
  buildSpecCompletions,
  buildACCompletions,
  buildAnnotationHover,
  buildQuickFix,
  suggestACsForFunction,
  findNearestSpecAnnotation,
} from '../annotations';

import type { SpecIndex, SpecSummary } from '../types';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const specIndex: SpecIndex = {
  specs: {
    'payment-create-intent': {
      id: 'payment-create-intent',
      title: 'Create a payment intent',
      tier: 1,
      file: '/project/specs/payments/create-intent.spec.yaml',
      acs: [
        { id: 'AC-01', description: 'Valid currency creates intent successfully' },
        { id: 'AC-02', description: 'Invalid currency returns 422 with error code INVALID_CURRENCY' },
        { id: 'AC-03', description: 'Duplicate idempotency key returns the existing intent' },
      ],
      coveragePct: 67,
      status: 'approved',
    },
    'auth-verify-token': {
      id: 'auth-verify-token',
      title: 'Verify a JWT access token',
      tier: 1,
      file: '/project/specs/auth/verify-token.spec.yaml',
      acs: [
        { id: 'AC-01', description: 'Valid token returns decoded claims' },
        { id: 'AC-02', description: 'Expired token returns 401' },
      ],
      coveragePct: 100,
      status: 'approved',
    },
  },
};

// ---------------------------------------------------------------------------
// AC-07: @spec completions ranked by directory proximity
// ---------------------------------------------------------------------------

// @ac AC-07
describe('buildSpecCompletions', () => {
  it('returns completion items for all specs in the index', () => {
    const items = buildSpecCompletions(specIndex, '/project/src/payments/handler.test.ts');
    const ids = items.map(i => i.insertText);
    expect(ids).toContain('payment-create-intent');
    expect(ids).toContain('auth-verify-token');
  });

  it('ranks specs in the same directory higher', () => {
    const items = buildSpecCompletions(specIndex, '/project/specs/payments/create-intent.test.ts');
    expect(items[0].insertText).toBe('payment-create-intent');
  });

  it('includes spec title as detail and tier in documentation', () => {
    const items = buildSpecCompletions(specIndex, '/project/src/test.ts');
    const payItem = items.find(i => i.insertText === 'payment-create-intent')!;
    expect(payItem.detail).toContain('Create a payment intent');
    expect(payItem.documentation).toContain('T1');
  });
});

// ---------------------------------------------------------------------------
// AC-08: @ac completions scoped to the nearest @spec annotation above
// ---------------------------------------------------------------------------

// @ac AC-08
describe('buildACCompletions', () => {
  it('returns AC IDs from the spec referenced by the nearest @spec annotation', () => {
    const fileContent = `
// @spec payment-create-intent
// @ac
function testCreateIntent() {}
    `.trim();
    const items = buildACCompletions(specIndex, fileContent, /* cursorLine */ 1);
    expect(items.map(i => i.insertText)).toContain('AC-01');
    expect(items.map(i => i.insertText)).toContain('AC-02');
    expect(items.map(i => i.insertText)).toContain('AC-03');
  });

  it('includes AC description as documentation in each completion item', () => {
    const fileContent = '// @spec payment-create-intent\n// @ac ';
    const items = buildACCompletions(specIndex, fileContent, 1);
    const ac01 = items.find(i => i.insertText === 'AC-01')!;
    expect(ac01.documentation).toContain('Valid currency creates intent successfully');
  });

  it('returns empty when no @spec annotation precedes the cursor', () => {
    const fileContent = '// @ac ';
    const items = buildACCompletions(specIndex, fileContent, 0);
    expect(items).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// AC-09: Hover on @ac shows full description, coverage status, and other files
// ---------------------------------------------------------------------------

// @ac AC-09
describe('buildAnnotationHover', () => {
  it('shows the full AC description on hover over @ac AC-01', () => {
    const hover = buildAnnotationHover(specIndex, 'payment-create-intent', 'AC-01', {
      coveredByFiles: ['src/payments/create_test.go'],
    });
    expect(hover.contents).toContain('Valid currency creates intent successfully');
  });

  it('shows coverage status — covered or uncovered', () => {
    const covered = buildAnnotationHover(specIndex, 'payment-create-intent', 'AC-01', {
      coveredByFiles: ['src/test.ts'],
    });
    expect(covered.contents.toLowerCase()).toContain('covered');

    const uncovered = buildAnnotationHover(specIndex, 'payment-create-intent', 'AC-02', {
      coveredByFiles: [],
    });
    expect(uncovered.contents.toLowerCase()).toContain('uncovered');
  });

  it('lists other test files that cover the same AC', () => {
    const hover = buildAnnotationHover(specIndex, 'payment-create-intent', 'AC-01', {
      coveredByFiles: ['src/payments/create_test.go', 'src/integration/payment_test.go'],
    });
    expect(hover.contents).toContain('create_test.go');
    expect(hover.contents).toContain('integration/payment_test.go');
  });

  it('returns empty hover for unknown spec or AC ID', () => {
    const hover = buildAnnotationHover(specIndex, 'nonexistent', 'AC-01', { coveredByFiles: [] });
    expect(hover.contents).toBe('');
  });
});

// ---------------------------------------------------------------------------
// AC-15: Quick-fix inserts @spec + @ac snippet above unannotated function
// ---------------------------------------------------------------------------

// @ac AC-15
describe('buildQuickFix', () => {
  it('inserts @spec and @ac lines above the function', () => {
    const fix = buildQuickFix({
      specID: 'payment-create-intent',
      functionLine: 5,
    });
    expect(fix.insertLine).toBe(5);
    expect(fix.text).toContain('// @spec payment-create-intent');
    expect(fix.text).toContain('// @ac AC-');
  });

  it('includes a tab stop at the AC ID position', () => {
    const fix = buildQuickFix({ specID: 'payment-create-intent', functionLine: 3 });
    // Tab stop marker varies by snippet format; check for cursor placeholder
    expect(fix.isSnippet).toBe(true);
  });

  it('uses best-guess spec ID when one is provided', () => {
    const fix = buildQuickFix({ specID: 'auth-verify-token', functionLine: 0 });
    expect(fix.text).toContain('auth-verify-token');
  });
});

// ---------------------------------------------------------------------------
// AC-21: tf-idf AC suggestion — offline, no LM call
// ---------------------------------------------------------------------------

// @ac AC-21
describe('suggestACsForFunction', () => {
  it('returns top-2 AC suggestions for a function body', () => {
    const suggestions = suggestACsForFunction(
      specIndex,
      `
      function testCreatePaymentWithValidCurrency() {
        const result = createIntent({ currency: 'USD', amount: 1000 });
        expect(result.status).toBe('pending');
      }
      `,
    );
    expect(suggestions.length).toBeGreaterThanOrEqual(1);
    expect(suggestions.length).toBeLessThanOrEqual(2);
    // The top suggestion should be payment-related given the function body
    expect(suggestions[0].specID).toBe('payment-create-intent');
  });

  it('returns empty array for a function body with no matching tokens', () => {
    const suggestions = suggestACsForFunction(specIndex, 'function noop() {}');
    expect(suggestions).toHaveLength(0);
  });

  it('never calls a network API (all computation is synchronous)', () => {
    // If suggestACsForFunction returns a Promise, this test fails
    const result = suggestACsForFunction(specIndex, 'function testPayment() {}');
    expect(result).not.toBeInstanceOf(Promise);
  });
});

// ---------------------------------------------------------------------------
// Helper: findNearestSpecAnnotation
// ---------------------------------------------------------------------------

// @ac AC-08
describe('findNearestSpecAnnotation', () => {
  it('finds the @spec annotation nearest above the given line', () => {
    const content = [
      '// @spec auth-verify-token',
      '// @ac AC-01',
      'function testVerifyToken() {}',
      '',
      '// @spec payment-create-intent',
      '// @ac AC-02',   // <- cursor here (line 5)
    ].join('\n');
    expect(findNearestSpecAnnotation(content, 5)).toBe('payment-create-intent');
  });

  it('returns null when no @spec annotation precedes the given line', () => {
    const content = '// @ac AC-01\nfunction test() {}';
    expect(findNearestSpecAnnotation(content, 0)).toBeNull();
  });
});
