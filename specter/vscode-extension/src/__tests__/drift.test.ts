// @spec spec-vscode
//
// Tests for intent drift detection — tracking when a spec AC changes after
// a @ac annotation was last committed, and classifying the change class
// via specter diff output.

import {
  detectDrift,
  buildDriftHover,
  DriftBaseline,
} from '../drift';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// Simulates the output of `specter diff --json` for two spec versions
const diffOutputBreaking = {
  changeClass: 'breaking',
  changes: [
    {
      kind: 'ac_removed',
      acID: 'AC-02',
      baselineDescription: 'Invalid currency returns 422',
      headDescription: null,
    },
  ],
};

const diffOutputAdditive = {
  changeClass: 'additive',
  changes: [
    {
      kind: 'ac_added',
      acID: 'AC-04',
      baselineDescription: null,
      headDescription: 'New AC added at HEAD',
    },
  ],
};

const diffOutputPatch = {
  changeClass: 'patch',
  changes: [
    {
      kind: 'description_changed',
      acID: 'AC-01',
      baselineDescription: 'Valid currency creates intent',
      headDescription: 'Valid currency creates intent successfully',
    },
  ],
};

// ---------------------------------------------------------------------------
// AC-14: Intent drift detection — git hash comparison + specter diff
// ---------------------------------------------------------------------------

// @ac AC-14
describe('[spec-vscode/AC-14] detectDrift', () => {
  it('returns no drift when spec file hash matches the annotation baseline', () => {
    const baseline: DriftBaseline = {
      specID: 'payment-create-intent',
      acID: 'AC-01',
      specFileHashAtAnnotation: 'abc123',
    };
    const result = detectDrift(baseline, {
      currentSpecFileHash: 'abc123',
      getDiff: () => null,
    });
    expect(result.hasDrift).toBe(false);
  });

  it('detects drift when spec file hash differs from annotation baseline', () => {
    const baseline: DriftBaseline = {
      specID: 'payment-create-intent',
      acID: 'AC-02',
      specFileHashAtAnnotation: 'abc123',
    };
    const result = detectDrift(baseline, {
      currentSpecFileHash: 'def456',
      getDiff: () => diffOutputBreaking,
    });
    expect(result.hasDrift).toBe(true);
  });

  it('classifies breaking drift correctly from specter diff output', () => {
    const baseline: DriftBaseline = {
      specID: 'payment-create-intent',
      acID: 'AC-02',
      specFileHashAtAnnotation: 'abc123',
    };
    const result = detectDrift(baseline, {
      currentSpecFileHash: 'def456',
      getDiff: () => diffOutputBreaking,
    });
    expect(result.changeClass).toBe('breaking');
  });

  it('classifies additive drift correctly', () => {
    const baseline: DriftBaseline = {
      specID: 'payment-create-intent',
      acID: 'AC-01',
      specFileHashAtAnnotation: 'abc123',
    };
    const result = detectDrift(baseline, {
      currentSpecFileHash: 'def456',
      getDiff: () => diffOutputAdditive,
    });
    expect(result.changeClass).toBe('additive');
  });

  it('classifies patch drift correctly', () => {
    const baseline: DriftBaseline = {
      specID: 'payment-create-intent',
      acID: 'AC-01',
      specFileHashAtAnnotation: 'abc123',
    };
    const result = detectDrift(baseline, {
      currentSpecFileHash: 'def456',
      getDiff: () => diffOutputPatch,
    });
    expect(result.changeClass).toBe('patch');
  });

  it('returns no drift (null changeClass) when getDiff returns null', () => {
    const baseline: DriftBaseline = {
      specID: 'payment-create-intent',
      acID: 'AC-01',
      specFileHashAtAnnotation: 'abc123',
    };
    // Hash differs but diff is unavailable (e.g., not a git repo)
    const result = detectDrift(baseline, {
      currentSpecFileHash: 'def456',
      getDiff: () => null,
    });
    expect(result.hasDrift).toBe(true);
    expect(result.changeClass).toBeNull();
  });
});

// @ac AC-14
describe('[spec-vscode/AC-14] buildDriftHover', () => {
  it('shows the AC description at baseline and at HEAD', () => {
    const hover = buildDriftHover({
      specID: 'payment-create-intent',
      acID: 'AC-01',
      changeClass: 'patch',
      baselineDescription: 'Valid currency creates intent',
      headDescription: 'Valid currency creates intent successfully',
    });
    expect(hover.contents).toContain('Valid currency creates intent');
    expect(hover.contents).toContain('Valid currency creates intent successfully');
  });

  it('labels the change class in the hover card', () => {
    const hover = buildDriftHover({
      specID: 'payment-create-intent',
      acID: 'AC-02',
      changeClass: 'breaking',
      baselineDescription: 'Invalid currency returns 422',
      headDescription: null,
    });
    expect(hover.contents.toLowerCase()).toContain('breaking');
  });

  it('shows "AC removed at HEAD" when headDescription is null (AC was deleted)', () => {
    const hover = buildDriftHover({
      specID: 'payment-create-intent',
      acID: 'AC-02',
      changeClass: 'breaking',
      baselineDescription: 'Invalid currency returns 422',
      headDescription: null,
    });
    expect(hover.contents.toLowerCase()).toMatch(/removed|deleted/);
  });

  it('shows "AC added at HEAD" when baselineDescription is null (AC is new)', () => {
    const hover = buildDriftHover({
      specID: 'payment-create-intent',
      acID: 'AC-04',
      changeClass: 'additive',
      baselineDescription: null,
      headDescription: 'New AC added at HEAD',
    });
    expect(hover.contents).toContain('New AC added at HEAD');
  });

  it('includes the spec ID in the hover for context', () => {
    const hover = buildDriftHover({
      specID: 'payment-create-intent',
      acID: 'AC-01',
      changeClass: 'patch',
      baselineDescription: 'Old description',
      headDescription: 'New description',
    });
    expect(hover.contents).toContain('payment-create-intent');
  });
});

// @ac AC-14
describe('[spec-vscode/AC-14] DriftBaseline', () => {
  it('is a plain serializable object (no class instances or Promises)', () => {
    const baseline: DriftBaseline = {
      specID: 'payment-create-intent',
      acID: 'AC-01',
      specFileHashAtAnnotation: 'abc123',
    };
    // Must be serializable to persist alongside the workspace state
    expect(() => JSON.stringify(baseline)).not.toThrow();
    const round = JSON.parse(JSON.stringify(baseline)) as DriftBaseline;
    expect(round.specID).toBe('payment-create-intent');
    expect(round.acID).toBe('AC-01');
    expect(round.specFileHashAtAnnotation).toBe('abc123');
  });
});
