import test from 'node:test';
import assert from 'node:assert/strict';
import { architectureGraph, fileGraph, projectOptions, reconcileNavigation, connectionId } from './architecture.js';

const graph = {
  projects: [{ id: 'frontend', name: 'frontend', path: 'frontend' }, { id: 'backend', name: 'lomah-nest', path: 'lomah-nest' }],
  nodes: [
    { id: 'frontend/main.ts', projectId: 'frontend' },
    { id: 'backend/main.ts', projectId: 'backend' },
    { id: 'backend/auth.ts', projectId: 'backend' },
    { id: 'backend/auth.spec.ts', projectId: 'backend', isTest: true }
  ],
  edges: [
    { from: 'backend/main.ts', to: 'backend/auth.ts', kind: 'imports' },
    { from: 'backend/auth.spec.ts', to: 'backend/auth.ts', kind: 'imports' }
  ],
  architecture: {
    modules: [
      { id: 'ui', projectId: 'frontend', memberIds: ['frontend/main.ts'], entryPoints: ['frontend/main.ts'], fileCount: 1, changedCount: 0 },
      { id: 'app', projectId: 'backend', memberIds: ['backend/main.ts', 'backend/auth.ts'], entryPoints: ['backend/main.ts'], fileCount: 2, changedCount: 0 },
      { id: 'tests', projectId: 'backend', memberIds: ['backend/auth.spec.ts'], entryPoints: [], isTest: true, fileCount: 1, changedCount: 1 }
    ],
    edges: [{ from: 'tests', to: 'app', kind: 'imports', count: 1, evidence: [{ from: 'backend/auth.spec.ts', to: 'backend/auth.ts', kind: 'imports' }] }]
  }
};

test('project filtering keeps backend TypeScript and never creates connections', () => {
  assert.deepEqual(projectOptions(graph).map((p) => p.label), ['All projects', 'frontend', 'lomah-nest']);
  assert.equal(fileGraph(graph, 'backend').nodes.length, 3);
  assert.equal(architectureGraph(graph).edges.length, 1);
  assert.equal(architectureGraph(graph).nodes.length, 3); // disconnected UI stays visible
  assert.deepEqual(architectureGraph(graph, 'backend').nodes.map((n) => n.id), ['app', 'tests']);
  assert.equal(architectureGraph(graph, 'backend').edges[0].evidence[0].from, 'backend/auth.spec.ts');
});

test('drill-down keeps file identities and internal dependencies; return restores overview', () => {
  assert.deepEqual(fileGraph(graph, 'backend', 'app').nodes.filter((n) => !n.context).map((n) => n.id), ['backend/main.ts', 'backend/auth.ts']);
  assert.equal(fileGraph(graph, 'backend', 'app').edges.length, 2);
  const tests = fileGraph(graph, 'backend', 'tests');
  assert.equal(tests.nodes.find((n) => !n.context).isTest, true);
  assert.equal(tests.nodes.find((n) => n.id === 'backend/auth.ts').context, true);
  assert.equal(tests.edges[0].from, 'backend/auth.spec.ts');
  assert.equal(architectureGraph(graph, 'backend').nodes.length, 2);
});

test('live updates preserve surviving selections and clear deleted groups and edges', () => {
  const state = { scope: 'backend', moduleId: '', selectedId: 'tests', edgeId: connectionId(graph.architecture.edges[0]), architecture: true };
  assert.equal(reconcileNavigation(graph, state).selectedId, 'tests');
  const changed = { ...graph, architecture: { modules: [graph.architecture.modules[1]], edges: [] } };
  assert.deepEqual(reconcileNavigation(changed, state), { scope: 'backend', moduleId: '', selectedId: '', edgeId: '' });
  assert.equal(reconcileNavigation(changed, { ...state, architecture: false, moduleId: 'tests' }).moduleId, '');
  assert.equal(reconcileNavigation({ projects: [], nodes: [] }, state).scope, 'all');
});

test('duplicate project names use relative paths', () => {
  assert.deepEqual(projectOptions({ projects: [{ id: 'a', name: 'api', path: 'one/api' }, { id: 'b', name: 'api', path: 'two/api' }] }).slice(1).map((p) => p.label), ['one/api', 'two/api']);
});
