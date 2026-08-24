<script>
  import { onMount } from 'svelte';
  import { Background, BackgroundVariant, Controls, SvelteFlow } from '@xyflow/svelte';
  import '@xyflow/svelte/dist/style.css';
  import CodeCard from './CodeCard.svelte';

  let nodes = [];
  let edges = [];
  const nodeTypes = { code: CodeCard };

  let graph = null;
  let selectedId = '';
  let view = 'changes';
  let scope = 'all';
  let period = '30';
  let loading = true;
  let connected = false;
  let notice = '';
  let activityLog = [];
  const viewOptions = [
    { id: 'calls', label: 'What calls what' },
    { id: 'changes', label: 'Changes together' },
    { id: 'both', label: 'Both' }
  ];
  const periodOptions = [
    { id: '30', label: '30d' },
    { id: '90', label: '90d' },
    { id: 'all', label: 'All' }
  ];
  const scopeOptions = [
    { id: 'all', label: 'All' },
    { id: 'frontend', label: 'Frontend' },
    { id: 'backend', label: 'Backend' }
  ];

  $: selected = nodes.find((node) => node.id === selectedId)?.data ?? graph?.nodes.find((node) => node.id === selectedId);
  $: changedNodes = graph?.nodes.filter((node) => node.change) ?? [];
  $: otherChangeCount = graph?.otherChanges.length ?? 0;
  $: changedCount = changedNodes.length + otherChangeCount;
  $: sourceFileCount = scopedSourceNodes(graph?.nodes ?? []).length;
  $: connectionCount = edges.length;
  $: selectedCommitCount = activityCount(selected?.activity);
  $: timelineMax = Math.max(1, ...(graph?.activity ?? []).map((bucket) => bucket.count));
  $: dependsOn = relatedNodes('out');
  $: usedBy = relatedNodes('in');
  $: connectedFiles = [...dependsOn, ...usedBy].slice(0, 4);

  function reviewGraph(apiNodes, apiEdges) {
    let roots = apiNodes.filter((node) => node.isRoot);
    if (roots.length === 0 && apiNodes.length > 0) {
      const incoming = new Set(apiEdges.map((edge) => edge.to));
      roots = apiNodes.filter((node) => !incoming.has(node.id) && !node.id.endsWith('_test.go'));
      if (roots.length === 0) roots = [apiNodes[0]];
    }

    const visibleIds = new Set([
      ...roots.map((node) => node.id),
      ...apiNodes.filter((node) => node.change).map((node) => node.id)
    ]);
    const visibleNodes = apiNodes.filter((node) => visibleIds.has(node.id));
    return {
      nodes: visibleNodes,
      edges: collapseHiddenPaths(apiEdges, visibleIds)
    };
  }

  function collapseHiddenPaths(apiEdges, visibleIds) {
    const outgoing = new Map();
    const directPairs = new Set();
    const result = [];

    for (const edge of apiEdges) {
      const neighbors = outgoing.get(edge.from) ?? [];
      neighbors.push(edge);
      outgoing.set(edge.from, neighbors);

      if (visibleIds.has(edge.from) && visibleIds.has(edge.to)) {
        directPairs.add(`${edge.from}:${edge.to}`);
        result.push(edge);
      }
    }

    const indirectPairs = new Set();
    for (const source of visibleIds) {
      const queue = [...(outgoing.get(source) ?? [])];
      const visitedHidden = new Set();

      for (let index = 0; index < queue.length; index += 1) {
        const target = queue[index].to;
        if (visibleIds.has(target)) {
          const pair = `${source}:${target}`;
          if (source !== target && !directPairs.has(pair) && !indirectPairs.has(pair)) {
            indirectPairs.add(pair);
            result.push({ from: source, to: target, kind: 'indirect' });
          }
          continue;
        }
        if (visitedHidden.has(target)) continue;
        visitedHidden.add(target);
        queue.push(...(outgoing.get(target) ?? []));
      }
    }

    return result.sort((left, right) =>
      left.from.localeCompare(right.from) ||
      left.to.localeCompare(right.to) ||
      left.kind.localeCompare(right.kind)
    );
  }

  function visibleGraph(apiNodes, apiEdges) {
    const scoped = buildScopedGraph(apiNodes, apiEdges);
    if (view === 'calls' || view === 'both') return scoped;
    return reviewGraph(scoped.nodes, scoped.edges);
  }

  function buildScopedGraph(apiNodes, apiEdges) {
    const scopedNodes = scopedSourceNodes(apiNodes);
    const scopedIds = new Set(scopedNodes.map((node) => node.id));
    const scopedEdges = apiEdges.filter((edge) => scopedIds.has(edge.from) && scopedIds.has(edge.to));
    const resultNodes = [...scopedNodes];
    const resultEdges = [...scopedEdges];
    const categories = [...new Set(scopedNodes.map(nodeArea))].sort();

    for (const category of categories) {
      const parent = categoryNode(category);
      resultNodes.push(parent);
      for (const root of categoryRoots(scopedNodes, scopedEdges, category)) {
        resultEdges.push({ from: parent.id, to: root.id, kind: 'category' });
      }
    }

    if (scope === 'all' && categories.includes('frontend') && categories.includes('backend')) {
      resultEdges.push({ from: '__frontend__', to: '__backend__', kind: 'bridge' });
    }

    return { nodes: resultNodes, edges: resultEdges };
  }

  function scopedSourceNodes(apiNodes) {
    if (scope === 'all') return apiNodes;
    return apiNodes.filter((node) => nodeArea(node) === scope);
  }

  function nodeArea(node) {
    const id = String(node.id ?? '').toLowerCase();
    const language = String(node.language ?? '').toLowerCase();
    if (
      id.startsWith('frontend/') ||
      ['svelte', 'javascript', 'typescript', 'css', 'html', 'vue'].includes(language)
    ) {
      return 'frontend';
    }
    return 'backend';
  }

  function categoryNode(category) {
    const frontend = category === 'frontend';
    const childCount = scopedSourceNodes(graph?.nodes ?? []).filter((node) => nodeArea(node) === category).length;
    return {
      id: frontend ? '__frontend__' : '__backend__',
      label: frontend ? 'Frontend' : 'Backend',
      language: 'Scope',
      kind: 'folder',
      description: frontend
        ? `${childCount} client-side source files and UI entry points.`
        : `${childCount} server-side source files and analysis/map logic.`,
      isRoot: true,
      openable: false,
      area: category,
      activity: { commits30: 0, commits90: 0, commitsAll: 0, people: 0, recentCommits: [] }
    };
  }

  function categoryRoots(apiNodes, apiEdges, category) {
    const categoryNodes = apiNodes.filter((node) => nodeArea(node) === category);
    const categoryIds = new Set(categoryNodes.map((node) => node.id));
    const incoming = new Set(apiEdges.filter((edge) => categoryIds.has(edge.from) && categoryIds.has(edge.to)).map((edge) => edge.to));
    const roots = categoryNodes.filter((node) => node.isRoot || !incoming.has(node.id));
    return (roots.length ? roots : categoryNodes.slice(0, 4)).slice(0, 6);
  }

  function layoutNodes(apiNodes, apiEdges) {
    const previousPositions = new Map(nodes.map((node) => [node.id, node.position]));
    const incoming = new Map(apiNodes.map((node) => [node.id, 0]));
    const outgoing = new Map(apiNodes.map((node) => [node.id, []]));

    for (const edge of apiEdges.filter((edge) => edge.kind !== 'bridge')) {
      incoming.set(edge.to, (incoming.get(edge.to) ?? 0) + 1);
      outgoing.get(edge.from)?.push(edge.to);
    }

    const roots = apiNodes.filter((node) => node.isRoot).map((node) => node.id);
    if (roots.length === 0) {
      roots.push(...apiNodes.filter((node) => incoming.get(node.id) === 0).map((node) => node.id));
    }
    if (roots.length === 0 && apiNodes[0]) roots.push(apiNodes[0].id);

    const depths = new Map();
    const queue = roots.map((id) => ({ id, depth: 0 }));
    for (let index = 0; index < queue.length; index += 1) {
      const current = queue[index];
      if (depths.has(current.id)) continue;
      depths.set(current.id, current.depth);
      for (const child of outgoing.get(current.id) ?? []) {
        if (!depths.has(child)) queue.push({ id: child, depth: current.depth + 1 });
      }
    }

    const fallbackDepth = Math.max(0, ...depths.values()) + 1;
    const nodesByDepth = new Map();
    for (const node of apiNodes) {
      const depth = depths.get(node.id) ?? fallbackDepth;
      const level = nodesByDepth.get(depth) ?? [];
      level.push(node);
      nodesByDepth.set(depth, level);
    }

    const result = [];
    const horizontalGap = 320;
    const verticalGap = 155;
    for (const [depth, level] of [...nodesByDepth.entries()].sort(([left], [right]) => left - right)) {
      level.forEach((node, index) => {
        const isScopeNode = String(node.id).startsWith('__');
        result.push({
          id: node.id,
          type: 'code',
          data: node,
          position: previousPositions.get(node.id) ?? {
            x: isScopeNode ? 0 : (depth + 1) * horizontalGap,
            y: isScopeNode ? (node.id === '__frontend__' ? 0 : 190) : index * verticalGap
          },
          ariaLabel: `${node.label}: ${node.description}`
        });
      });
    }
    return result;
  }

  function renderGraph(nextGraph = graph) {
    if (!nextGraph) return;
    const review = visibleGraph(nextGraph.nodes, nextGraph.edges);
    nodes = layoutNodes(review.nodes, review.edges);
    edges = review.edges.map((edge) => ({
      id: `${edge.from}:${edge.to}:${edge.kind}`,
      source: edge.from,
      target: edge.to,
      type: 'bezier',
      interactionWidth: 28,
      class: [
        edge.kind === 'indirect' ? 'indirect-edge' : edge.kind === 'bridge' ? 'bridge-edge' : edge.kind === 'category' ? 'category-edge' : 'code-edge',
        selectedId && (edge.from === selectedId || edge.to === selectedId) ? 'active-edge' : ''
      ].filter(Boolean).join(' ')
    }));
    if (!review.nodes.some((node) => node.id === selectedId)) {
      selectedId = review.nodes.find((node) => node.change)?.id ?? review.nodes[0]?.id ?? '';
    }
  }

  function setView(nextView) {
    view = nextView;
    renderGraph();
  }

  function setScope(nextScope) {
    scope = nextScope;
    selectedId = '';
    renderGraph();
  }

  function addActivity(message) {
    activityLog = [
      { at: new Date().toLocaleTimeString(), message },
      ...activityLog
    ].slice(0, 8);
  }

  function formatGeneratedAt(value) {
    if (!value) return 'waiting';
    return relativeTime(value);
  }

  function activityCount(activity = null) {
    if (!activity) return 0;
    if (period === '90') return activity.commits90 ?? 0;
    if (period === 'all') return activity.commitsAll ?? 0;
    return activity.commits30 ?? 0;
  }

  function relatedNodes(direction) {
    if (!graph || !selectedId) return [];
    const byId = new Map(graph.nodes.map((node) => [node.id, node]));
    const ids = [];
    for (const edge of graph.edges) {
      if (direction === 'out' && edge.from === selectedId) ids.push(edge.to);
      if (direction === 'in' && edge.to === selectedId) ids.push(edge.from);
    }
    return [...new Set(ids)]
      .map((id) => byId.get(id))
      .filter(Boolean)
      .sort((left, right) => activityCount(right.activity) - activityCount(left.activity));
  }

  function relativeTime(value) {
    if (!value) return 'never';
    const seconds = Math.max(1, Math.floor((Date.now() - new Date(value).getTime()) / 1000));
    if (seconds < 60) return 'just now';
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes} min ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours} hours ago`;
    const days = Math.floor(hours / 24);
    if (days < 30) return `${days} days ago`;
    const months = Math.floor(days / 30);
    if (months < 12) return `${months} months ago`;
    return `${Math.floor(months / 12)} years ago`;
  }

  function barHeight(count) {
    return `${Math.max(8, Math.round((count / timelineMax) * 112))}px`;
  }

  function relationWidth(node) {
    const max = Math.max(1, selectedCommitCount, ...connectedFiles.map((item) => activityCount(item.activity)));
    return `${Math.max(12, Math.round((activityCount(node.activity) / max) * 100))}%`;
  }

  function selectedLead() {
    if (!selected) return 'Select a file to inspect its role, activity, and connections.';
    if (selected.change) return selected.change.summary.changed;
    if (selected.activity?.commits30 > 0) return `Active source file with ${selected.activity.commits30} commits in the last 30 days.`;
    return selected.description;
  }

  async function loadGraph() {
    try {
      const response = await fetch('/api/graph', { cache: 'no-store' });
      if (!response.ok) throw new Error(`Graph request failed (${response.status})`);
      const nextGraph = await response.json();
      const previousRevision = graph?.revision;
      graph = nextGraph;
      renderGraph(nextGraph);
      loading = false;
      notice = '';
      if (previousRevision !== nextGraph.revision) {
        addActivity(`Loaded repository map revision ${nextGraph.revision}`);
      }
    } catch (error) {
      loading = false;
      notice = error instanceof Error ? error.message : String(error);
    }
  }

  async function openNode(id) {
    if (!id) return;
    try {
      const response = await fetch('/api/open', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id })
      });
      if (!response.ok) throw new Error((await response.text()).trim() || 'Could not open file');
      notice = `Opened ${id}`;
    } catch (error) {
      notice = error instanceof Error ? error.message : String(error);
    }
  }

  function handleNodeClick({ node, event }) {
    selectedId = node.id;
    renderGraph();
    if ('detail' in event && event.detail >= 2) openNode(node.id);
  }

  onMount(() => {
    loadGraph();
    const events = new EventSource('/events');
    events.addEventListener('ready', () => {
      connected = true;
      addActivity('Connected to the live repository watcher');
    });
    events.addEventListener('graph-changed', (event) => {
      const payload = JSON.parse(event.data);
      connected = true;
      addActivity(`Detected repository changes for revision ${payload.revision}`);
      loadGraph();
    });
    events.addEventListener('analysis-error', (event) => {
      const payload = JSON.parse(event.data);
      notice = `Keeping the last valid map: ${payload.error}`;
      addActivity(`Analysis error: ${payload.error}`);
    });
    events.onerror = () => connected = false;
    return () => events.close();
  });
</script>

<svelte:head>
  <meta name="description" content="A live, local map of the current Git repository" />
  <title>{graph?.repository ?? 'gitcs'} · live map</title>
</svelte:head>

<main>
  <header>
    <div class="brand">
      <span class="mark"></span>
      <strong>codemap</strong>
      <span>{graph?.repository ?? 'loading repository'}</span>
      <em>{graph?.branch ?? 'main'}</em>
    </div>
    <div class="sync" class:online={connected}>
      <span>last sync {formatGeneratedAt(graph?.generatedAt)}</span><i></i>
    </div>
  </header>

  <div class="dashboard">
    <section class="left-panel">
      <nav class="toolbar" aria-label="Map controls">
        <div class="control-group">
          <span>Show</span>
          <div>
            {#each viewOptions as option}
              <button class:active={view === option.id} onclick={() => setView(option.id)}>{option.label}</button>
            {/each}
          </div>
        </div>
        <div class="control-group scope-filter">
          <span>Scope</span>
          <div>
            {#each scopeOptions as option}
              <button class:active={scope === option.id} onclick={() => setScope(option.id)}>{option.label}</button>
            {/each}
          </div>
        </div>
        <div class="control-group activity-filter">
          <span>Change activity</span>
          <div>
            {#each periodOptions as option}
              <button class:active={period === option.id} onclick={() => period = option.id}>{option.label}</button>
            {/each}
          </div>
        </div>
      </nav>

      {#if notice}<div class="notice" role="status">{notice}</div>{/if}

      <section class="map-shell" aria-label="Repository code map">
        <div class="map-legend" aria-label="Connection colors">
          <span><i class="direct"></i>code link</span>
          <span><i class="bridge"></i>frontend/backend</span>
          <span><i class="related"></i>collapsed path</span>
        </div>
        {#if loading}
          <div class="loading">Building the repository map...</div>
        {:else if nodes.length === 0}
          <div class="loading">No supported source files found.</div>
        {:else}
          <SvelteFlow
            bind:nodes
            bind:edges
            {nodeTypes}
            fitView
            fitViewOptions={{ padding: 0.22, maxZoom: 0.95 }}
            minZoom={0.32}
            maxZoom={1.8}
            onnodeclick={handleNodeClick}
            nodesConnectable={false}
            deleteKey={null}
            proOptions={{ hideAttribution: true }}
          >
            <Background variant={BackgroundVariant.Dots} gap={24} size={1} color="#172533" />
            <Controls position="bottom-left" />
          </SvelteFlow>
        {/if}
      </section>

      <section class="timeline" aria-label="Commits over time">
        <div class="timeline-head">
          <span>Commits over time</span>
          <em>{graph?.activity?.length ?? 0} weeks · live local Git history</em>
        </div>
        <div class="bars">
          {#each graph?.activity ?? [] as bucket, index}
            <button
              class:hot={index >= (graph?.activity?.length ?? 0) - 4}
              title={`${bucket.count} commits`}
              style={`height: ${barHeight(bucket.count)}`}
            >
              <span>{bucket.count}</span>
            </button>
          {/each}
        </div>
      </section>
    </section>

    <aside>
      {#if selected}
        <p class="eyebrow">Building the map</p>
        <h1>{selected.label}</h1>
        <code>{selected.id}</code>
        <p class="lead">{selectedLead()}</p>

        <div class="stat-grid">
          <div><strong>{selectedCommitCount}</strong><span>commits · {period === 'all' ? 'all time' : `${period} days`}</span></div>
          <div><strong>{selected.activity?.people ?? 0}</strong><span>people touching it</span></div>
          <div><strong>{relativeTime(selected.activity?.lastChangedAt)}</strong><span>last change</span></div>
        </div>

        {#if connectedFiles.length}
          <h3>Usually changes with</h3>
          <div class="relation-list">
            {#each connectedFiles as item}
              <button onclick={() => { selectedId = item.id; renderGraph(); }}>
                <span>{item.label}</span>
                <em>{activityCount(item.activity)} commits</em>
                <b><i style={`width: ${relationWidth(item)}`}></i></b>
              </button>
            {/each}
          </div>
          <p class="muted">These are directly connected files in the local code graph.</p>
        {/if}

        {#if selected.change}
          <h3>Previous vs now</h3>
          <div class="summary-block">
            <strong>Previously</strong><p>{selected.change.summary.previous}</p>
            <strong>Now</strong><p>{selected.change.summary.current}</p>
            <strong>Changed</strong><p>{selected.change.summary.changed}</p>
            <strong>Impact</strong><p>{selected.change.summary.impact}</p>
          </div>
          <div class="change-summary status-{selected.change.status.toLowerCase()}">
            <strong>{selected.change.status}</strong>
            <span>+{selected.change.additions} / -{selected.change.deletions}</span>
            <span>first change: line {selected.change.firstChangedLine}</span>
          </div>
        {/if}

        <h3>Recent commits</h3>
        <div class="commit-list">
          {#each selected.activity?.recentCommits ?? [] as commit}
            <div><b>{commit.hash}</b><span>{commit.message}</span><em>{commit.author || 'unknown'} · {relativeTime(commit.when)}</em></div>
          {:else}
            <p class="muted">No recent commits found for this file.</p>
          {/each}
        </div>

        <h3>Depends on</h3>
        <div class="chips">
          {#each dependsOn.slice(0, 6) as item}<button onclick={() => { selectedId = item.id; renderGraph(); }}>{item.label}</button>{:else}<span>None detected</span>{/each}
        </div>
        <h3>Used by</h3>
        <div class="chips">
          {#each usedBy.slice(0, 6) as item}<button onclick={() => { selectedId = item.id; renderGraph(); }}>{item.label}</button>{:else}<span>None detected</span>{/each}
        </div>

        <button class="open-button" disabled={!selected.openable} onclick={() => openNode(selected.id)}>Open in VS Code</button>
      {:else}
        <p class="eyebrow">Repository</p>
        <h1>{graph?.repository ?? 'Loading'}</h1>
        <p class="lead">Select a file to inspect activity, recent commits, and code connections.</p>
      {/if}
    </aside>
  </div>
</main>

<style>
  :global(*) { box-sizing: border-box; }
  :global(html) { height: 100%; overflow: hidden; background: #070d14; }
  :global(body) { margin: 0; min-width: 320px; height: 100%; overflow: hidden; color: #dce6ef; font-family: Inter, ui-sans-serif, system-ui, sans-serif; background: #070d14; }
  :global(button), :global(input) { font: inherit; }
  main { height: 100vh; height: 100dvh; min-height: 0; overflow: hidden; display: flex; flex-direction: column; }
  header { flex: 0 0 52px; height: 52px; padding: 0 18px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #172332; background: #090f17; }
  .brand, .sync, .toolbar, .control-group, .control-group > div, .map-legend { display: flex; align-items: center; }
  .brand { gap: 10px; }
  .brand strong { font-size: 0.9rem; letter-spacing: 0.02em; }
  .brand > span:last-of-type { color: #607184; font: 0.78rem ui-monospace, Consolas, monospace; }
  .brand em { padding: 5px 9px; border: 1px solid #1a2a3a; border-radius: 7px; color: #9dacbb; background: #0c1520; font: 700 0.72rem ui-monospace, Consolas, monospace; font-style: normal; }
  .mark { width: 10px; height: 10px; border-radius: 3px; background: #ff9f43; }
  .sync { gap: 7px; color: #708094; font-size: 0.72rem; }
  .sync i { width: 6px; height: 6px; border-radius: 50%; background: #f59e0b; }
  .sync.online i { background: #35d07f; box-shadow: 0 0 14px rgba(53, 208, 127, 0.7); }
  .dashboard { min-height: 0; flex: 1 1 auto; overflow: hidden; display: grid; grid-template-columns: minmax(0, 1fr) minmax(430px, 490px); }
  .left-panel { min-width: 0; min-height: 0; display: grid; grid-template-rows: 46px minmax(0, 1fr) 158px; border-right: 1px solid #172332; overflow: hidden; }
  .toolbar { gap: 12px; padding: 0 12px; border-bottom: 1px solid #121d2a; background: #080e15; overflow-x: auto; }
  .control-group { gap: 8px; }
  .control-group > span, .timeline-head span, .eyebrow, h3 { color: #65768a; font: 800 0.62rem ui-monospace, Consolas, monospace; letter-spacing: 0.14em; text-transform: uppercase; white-space: nowrap; }
  .control-group > div { gap: 2px; padding: 3px; border: 1px solid #182839; border-radius: 8px; background: #0b141f; }
  .control-group button { border: 0; border-radius: 6px; padding: 6px 9px; color: #8293a5; background: transparent; cursor: pointer; font-size: 0.72rem; font-weight: 750; line-height: 1; white-space: nowrap; }
  .control-group button.active { color: #dbe6ef; background: #223247; }
  .scope-filter, .activity-filter { margin-left: 4px; padding-left: 12px; border-left: 1px solid #182635; }
  .notice { padding: 9px 22px; color: #f6c56e; background: #211a10; font-size: 0.8rem; }
  .map-shell { position: relative; min-height: 0; background: radial-gradient(circle at 1px 1px, #132130 1px, transparent 0) 0 0 / 36px 36px, #0b1119; overflow: hidden; }
  .loading { position: absolute; inset: 0; display: grid; place-items: center; color: #708898; z-index: 2; }
  .map-legend { position: absolute; left: 16px; top: 16px; z-index: 4; gap: 12px; padding: 8px 10px; border: 1px solid #17283a; border-radius: 8px; color: #76889b; background: rgba(8, 14, 21, 0.84); font-size: 0.72rem; pointer-events: none; }
  .map-legend span { display: flex; align-items: center; gap: 6px; white-space: nowrap; }
  .map-legend i { width: 18px; height: 3px; border-radius: 999px; }
  .map-legend .direct { background: #37c4ee; }
  .map-legend .bridge { background: #d49a3c; }
  .map-legend .related { background: repeating-linear-gradient(90deg, #7d8794 0 5px, transparent 5px 9px); }
  .timeline { min-height: 0; padding: 13px 18px 16px; border-top: 1px solid #172332; background: #081018; overflow: hidden; }
  .timeline-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
  .timeline-head em { color: #596b7f; font: 0.7rem ui-monospace, Consolas, monospace; font-style: normal; }
  .bars { height: 96px; display: grid; grid-template-columns: repeat(24, minmax(10px, 1fr)); align-items: end; gap: 6px; }
  .bars button { min-width: 0; border: 0; border-radius: 5px 5px 2px 2px; background: #1a2634; cursor: pointer; position: relative; }
  .bars button.hot { background: linear-gradient(180deg, #ff9941, #e7a645); }
  .bars button span { position: absolute; inset: auto 0 100% 0; color: #708296; font-size: 0.65rem; opacity: 0; }
  .bars button:hover span { opacity: 1; }
  aside { min-width: 0; min-height: 0; overflow-y: auto; background: #080e15; }
  aside > * { margin-left: 22px; margin-right: 22px; }
  aside .eyebrow { margin-top: 20px; margin-bottom: 8px; }
  aside h1 { margin: 0 22px 4px; color: #eff5fb; font-size: 1.25rem; letter-spacing: 0; overflow-wrap: anywhere; }
  aside code { display: block; color: #607184; overflow-wrap: anywhere; font-size: 0.76rem; }
  .lead { margin-top: 14px; margin-bottom: 18px; color: #a6b4c3; font-size: 0.86rem; line-height: 1.38; }
  .stat-grid { margin: 0; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); border-top: 1px solid #172332; border-bottom: 1px solid #172332; }
  .stat-grid div { min-width: 0; padding: 16px 18px; border-right: 1px solid #172332; }
  .stat-grid div:last-child { border-right: 0; }
  .stat-grid strong { display: block; color: #f58a39; font-size: 1.32rem; }
  .stat-grid span { display: block; margin-top: 2px; color: #6f7f91; font-size: 0.7rem; line-height: 1.28; }
  h3 { margin-top: 20px; margin-bottom: 10px; }
  .relation-list { display: grid; gap: 9px; }
  .relation-list button { border: 0; padding: 0; display: grid; grid-template-columns: 1fr auto; gap: 6px 12px; color: #dbe4ed; background: transparent; text-align: left; cursor: pointer; }
  .relation-list em { color: #c3913b; font: 700 0.77rem ui-monospace, Consolas, monospace; font-style: normal; }
  .relation-list b { grid-column: 1 / -1; height: 5px; overflow: hidden; border-radius: 999px; background: #162330; }
  .relation-list i { display: block; height: 100%; border-radius: inherit; background: #c99840; }
  .muted { color: #728296; font-size: 0.75rem; line-height: 1.35; }
  .summary-block { display: grid; gap: 5px; padding: 11px; border: 1px solid #172637; border-radius: 8px; background: #0b141e; }
  .summary-block strong { color: #dce7ef; font-size: 0.72rem; }
  .summary-block p { margin: 0 0 5px; color: #8799aa; font-size: 0.75rem; line-height: 1.35; }
  .change-summary { display: grid; grid-template-columns: auto 1fr; gap: 3px 10px; margin-top: 12px; padding: 10px; border: 1px solid #5d461c; border-radius: 8px; background: #18160f; color: #b8a77c; font-size: 0.72rem; }
  .change-summary strong { grid-row: span 2; align-self: center; color: #fbbf24; font-size: 1.1rem; }
  .change-summary.status-a { border-color: #1f5a42; background: #0e1d18; }
  .change-summary.status-a strong { color: #4ade80; }
  .change-summary.status-d { border-color: #61303c; background: #211117; }
  .change-summary.status-d strong { color: #fb7185; }
  .commit-list { display: grid; gap: 10px; }
  .commit-list div { display: grid; grid-template-columns: 58px 1fr; gap: 3px 12px; }
  .commit-list b { color: #35b6d7; font: 800 0.82rem ui-monospace, Consolas, monospace; }
  .commit-list span { color: #c1ccd7; font-size: 0.78rem; }
  .commit-list em { grid-column: 2; color: #67798c; font-size: 0.7rem; font-style: normal; }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; }
  .chips button, .chips span { border: 1px solid #1d3143; border-radius: 7px; padding: 6px 8px; color: #92a4b5; background: #0b1520; font: 0.72rem ui-monospace, Consolas, monospace; }
  .chips button { cursor: pointer; }
  .open-button { width: calc(100% - 44px); margin: 22px 22px; border: 0; border-radius: 8px; padding: 9px 12px; color: #061018; background: #67e8f9; font-weight: 800; cursor: pointer; }
  button:disabled { opacity: 0.45; cursor: not-allowed; }
  :global(.svelte-flow) { background: transparent; }
  :global(.svelte-flow__edge-path) { stroke: rgba(73, 96, 114, 0.3); stroke-width: 1.4; stroke-linecap: round; stroke-linejoin: round; filter: none; }
  :global(.svelte-flow__edge.active-edge .svelte-flow__edge-path) { stroke: #37c4ee; stroke-width: 2.6; filter: drop-shadow(0 0 7px rgba(55, 196, 238, 0.42)); }
  :global(.svelte-flow__edge.category-edge .svelte-flow__edge-path) { stroke: rgba(55, 196, 238, 0.38); stroke-width: 1.7; }
  :global(.svelte-flow__edge.bridge-edge .svelte-flow__edge-path) { stroke: #d49a3c; stroke-width: 2.3; filter: drop-shadow(0 0 6px rgba(212, 154, 60, 0.34)); }
  :global(.svelte-flow__edge.indirect-edge .svelte-flow__edge-path) { stroke: rgba(151, 122, 76, 0.34); stroke-width: 1.3; stroke-dasharray: 7 10; }
  :global(.svelte-flow__edge.indirect-edge.active-edge .svelte-flow__edge-path) { stroke: #d49a3c; filter: drop-shadow(0 0 7px rgba(212, 154, 60, 0.38)); }
  :global(.svelte-flow__edge-text), :global(.svelte-flow__edge-textbg) { display: none; }
  :global(.svelte-flow__controls) { border: 1px solid #1d3143; border-radius: 8px; overflow: hidden; box-shadow: none; }
  :global(.svelte-flow__controls-button) { border: 0; border-bottom: 1px solid #1d3143; color: #a7bfce; background: #0f1b27; }
  :global(.svelte-flow__controls-button:hover) { background: #17293a; }
  @media (max-width: 980px) {
    .dashboard { grid-template-columns: 1fr; }
    aside { border-top: 1px solid #172332; }
    .left-panel { border-right: 0; }
    .toolbar { overflow-x: auto; }
  }
  @media (max-width: 680px) {
    header { align-items: flex-start; height: auto; padding: 16px; gap: 14px; flex-direction: column; }
    .left-panel { grid-template-rows: auto 64vh 180px; }
    .toolbar { align-items: flex-start; flex-direction: column; padding: 14px 16px; }
    .scope-filter, .activity-filter { margin-left: 0; padding-left: 0; border-left: 0; }
    .map-legend { display: none; }
    .stat-grid { grid-template-columns: 1fr; }
    .stat-grid div { border-right: 0; border-bottom: 1px solid #172332; }
  }
</style>
