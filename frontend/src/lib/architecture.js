export const areaColors = { frontend: '#4aa8ff', rust: '#f07178', backend: '#49c991', unknown: '#95a4b8' };

function fileArea(node, projects) {
  if (node.language === 'Rust') return 'rust';
  return projects.get(node.projectId)?.role ?? 'unknown';
}

export function projectOptions(graph) {
  const projects = graph?.projects ?? [];
  return [{ id: 'all', label: 'All projects' }, ...projects.map((project) => ({
    id: project.id,
    label: projects.filter((other) => other.name === project.name).length > 1 ? project.path : project.name
  }))];
}

export function architectureGraph(graph, projectId = 'all') {
  const projects = new Map((graph?.projects ?? []).map((project) => [project.id, project]));
  const files = new Map((graph?.nodes ?? []).map((node) => [node.id, node]));
  const nodes = (graph?.architecture?.modules ?? [])
    .filter((module) => projectId === 'all' || module.projectId === projectId)
    .map((module) => ({
      ...module, kind: 'module', language: module.isTest ? 'Tests' : 'Module',
      projectName: projects.get(module.projectId)?.name ?? module.projectId,
      area: module.memberIds.some((id) => files.get(id)?.language === 'Rust') ? 'rust' : projects.get(module.projectId)?.role ?? 'unknown',
      openable: false, isRoot: !module.isTest && module.entryPoints.length > 0,
      description: `${module.fileCount} ${module.isTest ? 'test' : 'source'} files · ${module.changedCount} changing`,
    }));
  const ids = new Set(nodes.map((node) => node.id));
  return { nodes, edges: (graph?.architecture?.edges ?? []).filter((edge) => ids.has(edge.from) && ids.has(edge.to)) };
}

export function fileGraph(graph, projectId = 'all', moduleId = '') {
  const projects = new Map((graph?.projects ?? []).map((project) => [project.id, project]));
  const module = (graph?.architecture?.modules ?? []).find((item) => item.id === moduleId);
  const members = module ? new Set(module.memberIds) : null;
  const context = new Set();
  if (members) {
    for (const edge of graph?.edges ?? []) {
      if (members.has(edge.from)) context.add(edge.to);
      if (members.has(edge.to)) context.add(edge.from);
    }
  }
  const nodes = (graph?.nodes ?? []).filter((node) =>
    (projectId === 'all' || node.projectId === projectId) && (!members || members.has(node.id) || context.has(node.id)))
    .map((node) => ({ ...node, area: fileArea(node, projects), context: Boolean(members && !members.has(node.id)) }));
  const ids = new Set(nodes.map((node) => node.id));
  return { nodes, edges: (graph?.edges ?? []).filter((edge) => ids.has(edge.from) && ids.has(edge.to) && (!members || members.has(edge.from) || members.has(edge.to))) };
}

export function reconcileNavigation(graph, { scope = 'all', moduleId = '', selectedId = '', edgeId = '', architecture = true }) {
  if (!projectOptions(graph).some((option) => option.id === scope)) scope = 'all';
  if (!(graph?.architecture?.modules ?? []).some((module) => module.id === moduleId && (scope === 'all' || module.projectId === scope))) moduleId = '';
  const visible = architecture ? architectureGraph(graph, scope) : fileGraph(graph, scope, moduleId);
  if (!visible.nodes.some((node) => node.id === selectedId)) selectedId = '';
  if (!visible.edges.some((edge) => connectionId(edge) === edgeId)) edgeId = '';
  return { scope, moduleId, selectedId, edgeId };
}

export function connectionId(edge) { return JSON.stringify([edge.from, edge.to, edge.kind]); }
