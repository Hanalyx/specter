// @spec spec-vscode
//
// Tests for constraint hover cards and go-to-definition providers.

import {
  buildConstraintHover,
  resolveDefinitionTarget,
} from '../navigation';

import type { SpecIndex } from '../types';

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
        { id: 'AC-02', description: 'Invalid currency returns 422' },
        { id: 'AC-03', description: 'Duplicate idempotency key returns existing intent' },
      ],
      constraints: [
        { id: 'C-01', description: 'MUST validate currency against ISO 4217' },
        { id: 'C-02', description: 'MUST reject duplicate idempotency keys with 422' },
        { id: 'C-03', description: 'MUST log payment attempts to the audit trail' },
      ],
      constraintReferences: {
        'C-01': ['AC-01', 'AC-02'],
        'C-02': ['AC-03'],
        // C-03 is intentionally unreferenced by any AC
      },
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
      constraints: [
        { id: 'C-01', description: 'MUST verify JWT signature using RS256' },
      ],
      constraintReferences: {
        'C-01': ['AC-01', 'AC-02'],
      },
      coveragePct: 100,
      status: 'approved',
    },
  },
};

// ---------------------------------------------------------------------------
// AC-06: Constraint hover card — description, referencing ACs, coverage status
// ---------------------------------------------------------------------------

// @ac AC-06
describe('buildConstraintHover', () => {
  it('shows the constraint description on hover over a constraint ID', () => {
    const hover = buildConstraintHover(specIndex, 'payment-create-intent', 'C-01', {
      coveredACIDs: ['AC-01'],
    });
    expect(hover.contents).toContain('MUST validate currency against ISO 4217');
  });

  it('lists the AC IDs that reference this constraint', () => {
    const hover = buildConstraintHover(specIndex, 'payment-create-intent', 'C-01', {
      coveredACIDs: ['AC-01'],
    });
    expect(hover.contents).toContain('AC-01');
    expect(hover.contents).toContain('AC-02');
  });

  it('indicates coverage status for each referencing AC', () => {
    const hover = buildConstraintHover(specIndex, 'payment-create-intent', 'C-01', {
      coveredACIDs: ['AC-01'], // AC-02 is not covered
    });
    // AC-01 is covered, AC-02 is not
    expect(hover.contents.toLowerCase()).toMatch(/ac-01.*covered|covered.*ac-01/i);
    expect(hover.contents.toLowerCase()).toMatch(/ac-02.*uncovered|uncovered.*ac-02/i);
  });

  it('warns prominently when no AC references the constraint', () => {
    const hover = buildConstraintHover(specIndex, 'payment-create-intent', 'C-03', {
      coveredACIDs: [],
    });
    expect(hover.contents).toContain('No AC references this constraint');
  });

  it('returns empty hover for unknown spec', () => {
    const hover = buildConstraintHover(specIndex, 'nonexistent-spec', 'C-01', {
      coveredACIDs: [],
    });
    expect(hover.contents).toBe('');
  });

  it('returns empty hover for unknown constraint ID', () => {
    const hover = buildConstraintHover(specIndex, 'payment-create-intent', 'C-99', {
      coveredACIDs: [],
    });
    expect(hover.contents).toBe('');
  });

  it('shows all referencing ACs when constraint is referenced by multiple', () => {
    const hover = buildConstraintHover(specIndex, 'payment-create-intent', 'C-01', {
      coveredACIDs: ['AC-01', 'AC-02'],
    });
    // Both AC-01 and AC-02 reference C-01
    expect(hover.contents).toContain('AC-01');
    expect(hover.contents).toContain('AC-02');
  });
});

// ---------------------------------------------------------------------------
// AC-10: Go-to-definition — depends_on.spec_id and references_constraints
// ---------------------------------------------------------------------------

// @ac AC-10
describe('resolveDefinitionTarget', () => {
  it('resolves a spec_id in depends_on to the target .spec.yaml file path', () => {
    const target = resolveDefinitionTarget(specIndex, {
      kind: 'spec_id',
      value: 'auth-verify-token',
      sourceFile: '/project/specs/payments/create-intent.spec.yaml',
    });
    expect(target).not.toBeNull();
    expect(target!.file).toBe('/project/specs/auth/verify-token.spec.yaml');
    expect(target!.line).toBe(0); // Navigate to top of file
  });

  it('returns null for an unknown spec_id', () => {
    const target = resolveDefinitionTarget(specIndex, {
      kind: 'spec_id',
      value: 'nonexistent-spec',
      sourceFile: '/project/specs/payments/create-intent.spec.yaml',
    });
    expect(target).toBeNull();
  });

  it('resolves a constraint ID in references_constraints to the constraint line in the current spec', () => {
    const target = resolveDefinitionTarget(specIndex, {
      kind: 'constraint_ref',
      value: 'C-02',
      sourceFile: '/project/specs/payments/create-intent.spec.yaml',
    });
    expect(target).not.toBeNull();
    expect(target!.file).toBe('/project/specs/payments/create-intent.spec.yaml');
    // Line should be a non-negative integer pointing within the spec file
    expect(target!.line).toBeGreaterThanOrEqual(0);
  });

  it('returns null for an unknown constraint ref in the current spec', () => {
    const target = resolveDefinitionTarget(specIndex, {
      kind: 'constraint_ref',
      value: 'C-99',
      sourceFile: '/project/specs/payments/create-intent.spec.yaml',
    });
    expect(target).toBeNull();
  });

  it('returns null when the source spec file is not in the index', () => {
    const target = resolveDefinitionTarget(specIndex, {
      kind: 'constraint_ref',
      value: 'C-01',
      sourceFile: '/project/specs/unknown/mystery.spec.yaml',
    });
    expect(target).toBeNull();
  });

  it('resolves spec_id to the exact file path registered in the index', () => {
    const target = resolveDefinitionTarget(specIndex, {
      kind: 'spec_id',
      value: 'payment-create-intent',
      sourceFile: '/project/specs/auth/verify-token.spec.yaml',
    });
    expect(target!.file).toBe('/project/specs/payments/create-intent.spec.yaml');
  });
});
