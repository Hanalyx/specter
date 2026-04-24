/** @type {import('jest').Config} */
module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  testMatch: ['**/__tests__/**/*.test.ts'],
  moduleFileExtensions: ['ts', 'js', 'json'],
  transform: {
    '^.+\\.ts$': ['ts-jest', {
      tsconfig: {
        module: 'commonjs',
        target: 'ES2020',
        strict: true,
        esModuleInterop: true,
        skipLibCheck: true,
      },
    }],
  },
  // jest-junit emits JUnit XML alongside the default console reporter
  // so `specter ingest --junit` can read the output in dogfood-strict.
  reporters: [
    'default',
    ['jest-junit', {
      outputDirectory: '.',
      outputName: 'junit.xml',
      // "<describe> > <it>" in testcase names so specter ingest's
      // regex matches [spec-vscode/AC-NN] placed in either describe or it titles.
      ancestorSeparator: ' > ',
      classNameTemplate: '{classname}',
      titleTemplate: '{title}',
    }],
  ],
};
