import { describe, expect, it } from 'vitest';
import { lintConfig } from '../src/lib/lint';

describe('image config lint', () => {
  it('recognizes registry login settings as pipeline configuration', () => {
    expect(lintConfig(`registries:
  registry.example.com:
    username: ci-user
    password_env: REGISTRY_TOKEN
stages: [build]
Build:
  stage: build
  image: registry.example.com/team/app:latest
  script: [echo hi]
`)).toEqual([]);
  });

  it.each([
    'registry.example.com/team/build:latest',
    '\n    name: registry.example.com/team/build:latest\n    force_pull: true',
  ])('accepts registry credentials alongside a job image: %s', (image) => {
    expect(lintConfig(`stages: [build]
Build:
  stage: build
  registries:
    registry.example.com:
      username: ci-user
      password_env: REGISTRY_TOKEN
  image: ${image}
  script: [echo hi]
`)).toEqual([]);
  });

  it.each([
    '  image: ubuntu',
    '  image: {name: ubuntu, force_pull: true}',
    '  image:\n    name: ubuntu\n    force_pull: true',
    '  image:\n    force_pull: false\n    name: ubuntu',
    '    image:\n        name: ubuntu\n        force_pull: true',
  ])('accepts %s', (image) => {
    expect(lintConfig(`stages: [build]\nBuild:\n  stage: build\n${image}\n  script: [echo hi]\n`)).toEqual([]);
  });

  it.each([
    '  image:',
    '  image:\n    force_pull: true',
    "  image:\n    name: ''\n    force_pull: false",
    '  image:\n    force_pull: true\n  variables:\n    name: ubuntu',
  ])('reports a missing image name in %s', (image) => {
    const issues = lintConfig(`stages: [build]\nBuild:\n  stage: build\n${image}\n  script: [echo hi]\n`);
    expect(issues).toEqual([{ line: 2, message: 'job "Build" has no image' }]);
  });

  it('continues checking the job fields after an image mapping', () => {
    expect(lintConfig(`stages: [build]
Build:
  image:
    name: ubuntu
    force_pull: true
  stage: deploy
  needs: missing
  script: [echo hi]
`)).toEqual([
      { line: 6, message: 'stage "deploy" is not in stages [build]' },
      { line: 7, message: 'needs "missing" — no such job' },
    ]);
  });
});
