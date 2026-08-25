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
  // Positions the user dragged by hand. Everything else is re-laid out on
  // every render so cards cannot pile up.
  let pinnedPositions = new Map();
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
  $: selectedLabel = selected?.label ?? 'this file';

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

  // --- Layout -------------------------------------------------------------
  // Cards are laid out in columns by dependency depth. Every card's height is
  // estimated from its own content, and a column stacks by accumulated height,
  // so two cards can never land on top of each other.

  const CARD_WIDTH = 230;
  const SCOPE_CARD_WIDTH = 250;
  const COLUMN_GAP = 96;
  const ROW_GAP = 30;
  const MAX_COLUMN_HEIGHT = 1500;

  function isScopeId(id) {
    return String(id ?? '').startsWith('__');
  }

  function cardWidth(node) {
    return isScopeId(node.id) ? SCOPE_CARD_WIDTH : CARD_WIDTH;
  }

  // Mirrors CodeCard.svelte: padding + topline + title + 2-line clamped
  // description + activity rail, plus a diff row when the file is changing.
  // Calibrated against rendered cards: a one-line title measures 123px, and a
  // diff row adds 20px. Titles wrap at ~26 characters inside a 230px card.
  // SAFETY_PAD absorbs font differences across machines -- this estimate must
  // never come in under the real height, or cards overlap again.
  const CARD_BASE_HEIGHT = 119;
  const TITLE_LINE_HEIGHT = 19;
  const DIFF_ROW_HEIGHT = 20;
  const SAFETY_PAD = 8;

  function cardHeight(node) {
    if (isScopeId(node.id)) return 132;
    const titleLines = Math.min(2, Math.ceil(String(node.label ?? '').length / 26) || 1);
    return (
      CARD_BASE_HEIGHT +
      (titleLines - 1) * TITLE_LINE_HEIGHT +
      (node.change ? DIFF_ROW_HEIGHT : 0) +
      SAFETY_PAD
    );
  }

  function buildAdjacency(apiNodes, apiEdges) {
    const incoming = new Map(apiNodes.map((node) => [node.id, []]));
    const outgoing = new Map(apiNodes.map((node) => [node.id, []]));
    for (const edge of apiEdges) {
      if (edge.kind === 'bridge') continue;
      if (!outgoing.has(edge.from) || !incoming.has(edge.to)) continue;
      outgoing.get(edge.from).push(edge.to);
      incoming.get(edge.to).push(edge.from);
    }
    return { incoming, outgoing };
  }

  function assignDepths(apiNodes, outgoing, incoming) {
    let roots = apiNodes.filter((node) => node.isRoot).map((node) => node.id);
    if (roots.length === 0) {
      roots = apiNodes.filter((node) => (incoming.get(node.id) ?? []).length === 0).map((node) => node.id);
    }
    if (roots.length === 0 && apiNodes[0]) roots = [apiNodes[0].id];

    const depths = new Map();
    const queue = roots.map((id) => ({ id, depth: 0 }));
    for (let index = 0; index < queue.length; index += 1) {
      const { id, depth } = queue[index];
      if (depths.has(id)) continue;
      depths.set(id, depth);
      for (const child of outgoing.get(id) ?? []) {
        if (!depths.has(child)) queue.push({ id: child, depth: depth + 1 });
      }
    }

    // Anything the walk never reached still needs a home; give it a column of
    // its own past the deepest reachable one rather than dropping it at 0.
    const orphanDepth = Math.max(0, ...depths.values()) + 1;
    for (const node of apiNodes) {
      if (!depths.has(node.id)) depths.set(node.id, orphanDepth);
    }
    return depths;
  }

  // Order each column by the average position of the parents already placed to
  // its left. Children end up next to their parents, which removes most of the
  // edge crossings that made the old grid unreadable.
  function orderByBarycenter(column, placedRows, incoming) {
    const scored = column.map((node, index) => {
      const parents = (incoming.get(node.id) ?? [])
        .map((parentId) => placedRows.get(parentId))
        .filter((row) => row !== undefined);
      const barycenter = parents.length
        ? parents.reduce((sum, row) => sum + row, 0) / parents.length
        : Number.POSITIVE_INFINITY;
      return { node, index, barycenter };
    });
    scored.sort((left, right) => left.barycenter - right.barycenter || left.index - right.index);
    return scored.map((entry) => entry.node);
  }

  // A column taller than the viewport can hold becomes several side-by-side
  // strips instead of one endless stack.
  function splitIntoStrips(column) {
    const strips = [[]];
    let height = 0;
    for (const node of column) {
      const nodeHeight = cardHeight(node) + ROW_GAP;
      if (height + nodeHeight > MAX_COLUMN_HEIGHT && strips[strips.length - 1].length) {
        strips.push([]);
        height = 0;
      }
      strips[strips.length - 1].push(node);
      height += nodeHeight;
    }
    return strips;
  }

  function layoutNodes(apiNodes, apiEdges) {
    const { incoming, outgoing } = buildAdjacency(apiNodes, apiEdges);
    const depths = assignDepths(apiNodes, outgoing, incoming);

    const byDepth = new Map();
    for (const node of apiNodes) {
      const depth = depths.get(node.id) ?? 0;
      if (!byDepth.has(depth)) byDepth.set(depth, []);
      byDepth.get(depth).push(node);
    }

    const positions = new Map();
    const placedRows = new Map();
    let x = 0;

    for (const [, column] of [...byDepth.entries()].sort(([left], [right]) => left - right)) {
      const ordered = orderByBarycenter(column, placedRows, incoming);
      for (const strip of splitIntoStrips(ordered)) {
        const stripHeight =
          strip.reduce((sum, node) => sum + cardHeight(node), 0) + Math.max(0, strip.length - 1) * ROW_GAP;
        const stripWidth = Math.max(...strip.map(cardWidth));

        let y = -stripHeight / 2;
        for (const node of strip) {
          // Narrow cards sit centred inside a column sized by its widest card.
          positions.set(node.id, { x: x + (stripWidth - cardWidth(node)) / 2, y });
          placedRows.set(node.id, y + cardHeight(node) / 2);
          y += cardHeight(node) + ROW_GAP;
        }
        x += stripWidth + COLUMN_GAP;
      }
    }

    return apiNodes.map((node) => ({
      id: node.id,
      type: 'code',
      data: node,
      // Drive the card highlight from our own selection, not the flow's, so the
      // highlighted card always matches the file in the inspector.
      selected: node.id === selectedId,
      // Only a card the user dragged keeps its manual spot; everything else is
      // re-laid out, so a view change can never drop new cards onto old ones.
      position: pinnedPositions.get(node.id) ?? positions.get(node.id) ?? { x: 0, y: 0 },
      ariaLabel: `${node.label}: ${node.description}`
    }));
  }

  // --- Connection strength -------------------------------------------------
  // How often two files were committed together. This is the number the edge
  // thickness encodes, and the only honest answer to "how connected are these".

  let commitSets = new Map();

  function rebuildCommitSets(apiNodes) {
    commitSets = new Map(
      apiNodes.map((node) => [node.id, new Set(node.activity?.commitHashes ?? [])])
    );
  }

  function coChangeCount(fromId, toId) {
    const from = commitSets.get(fromId);
    const to = commitSets.get(toId);
    if (!from || !to || from.size === 0 || to.size === 0) return 0;
    const [small, large] = from.size <= to.size ? [from, to] : [to, from];
    let shared = 0;
    for (const hash of small) if (large.has(hash)) shared += 1;
    return shared;
  }

  // Three named tiers so the legend can label what a thickness means.
  function strengthTier(count) {
    if (count >= 5) return 'strong';
    if (count >= 2) return 'medium';
    return 'weak';
  }

  function edgeRelation(edge) {
    if (!selectedId) return 'neutral';
    if (edge.from === selectedId) return 'outgoing';
    if (edge.to === selectedId) return 'incoming';
    return 'unrelated';
  }

  function edgeKindClass(kind) {
    return `${edgeKindKey(kind)}-edge`;
  }

  // Colour carries the kind of connection and never changes with selection, so
  // a line means something on its own before you click anything. Selection only
  // adds emphasis (glow) or removes it (dim) -- it never repaints a line.
  const KIND_COLORS = {
    code: '#4aa8c9',
    indirect: '#9b7fc4',
    bridge: '#d4923c',
    category: '#41647a'
  };

  function edgeKindKey(kind) {
    if (kind === 'indirect') return 'indirect';
    if (kind === 'bridge') return 'bridge';
    if (kind === 'category') return 'category';
    return 'code';
  }

  const RELATION_PAINT_ORDER = { unrelated: 0, neutral: 1, incoming: 2, outgoing: 3 };

  function renderGraph(nextGraph = graph) {
    if (!nextGraph) return;
    const review = visibleGraph(nextGraph.nodes, nextGraph.edges);

    // Resolve the selection first: both the card highlight and every edge
    // colour depend on it, so it has to settle before either is built.
    if (!review.nodes.some((node) => node.id === selectedId)) {
      selectedId = review.nodes.find((node) => node.change)?.id ?? review.nodes[0]?.id ?? '';
    }

    rebuildCommitSets(nextGraph.nodes);
    nodes = layoutNodes(review.nodes, review.edges);

    edges = review.edges
      .map((edge) => {
        const relation = edgeRelation(edge);
        const structural = edge.kind === 'category' || edge.kind === 'bridge';
        const coChange = structural ? 0 : coChangeCount(edge.from, edge.to);
        return {
          id: `${edge.from}:${edge.to}:${edge.kind}`,
          source: edge.from,
          target: edge.to,
          type: 'default',
          interactionWidth: 24,
          // Structural lines group the map; they are not dependencies, so they
          // get no arrowhead and no strength.
          markerEnd: structural
            ? undefined
            : { type: 'arrowclosed', width: 13, height: 13, color: KIND_COLORS[edgeKindKey(edge.kind)] },
          class: `${edgeKindClass(edge.kind)} strength-${strengthTier(coChange)} rel-${relation}`,
          data: { coChange, relation, kind: edge.kind },
          relation
        };
      })
      // Draw dimmed lines first and the selected file's lines last, so the
      // highlighted path is never buried under the rest of the graph.
      .sort((left, right) => RELATION_PAINT_ORDER[left.relation] - RELATION_PAINT_ORDER[right.relation]);
  }

  function setView(nextView) {
    view = nextView;
    // A different graph gets a fresh layout; keeping hand-dragged spots from the
    // previous one is what used to drop new cards on top of old ones.
    pinnedPositions = new Map();
    renderGraph();
  }

  function setScope(nextScope) {
    scope = nextScope;
    selectedId = '';
    pinnedPositions = new Map();
    renderGraph();
  }

  function handleNodeDragStop({ targetNode }) {
    if (!targetNode) return;
    pinnedPositions.set(targetNode.id, { ...targetNode.position });
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
        <div class="map-legend" aria-label="How to read the connections">
          <div class="legend-row">
            <span class="legend-title">An arrow points from a file to what it uses</span>
          </div>
          <div class="legend-row">
            <span><i class="swatch code"></i>uses directly</span>
            <span><i class="swatch indirect"></i>uses via files not shown</span>
          </div>
          <div class="legend-row">
            <span><i class="swatch bridge"></i>frontend ↔ backend</span>
            <span><i class="swatch category"></i>grouping, not a dependency</span>
          </div>
          <div class="legend-row">
            <span class="legend-label">changed together:</span>
            <span><i class="gauge weak"></i>rarely</span>
            <span><i class="gauge medium"></i>sometimes</span>
            <span><i class="gauge strong"></i>often</span>
          </div>
          {#if selected}
            <div class="legend-row legend-foot">Lit lines are {selectedLabel}'s own connections</div>
          {/if}
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
            onnodedragstop={handleNodeDragStop}
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
  .map-legend { position: absolute; left: 16px; top: 16px; z-index: 4; display: grid; gap: 6px; padding: 10px 12px; border: 1px solid #17283a; border-radius: 9px; color: #8b9dae; background: rgba(8, 14, 21, 0.88); backdrop-filter: blur(12px); font-size: 0.71rem; pointer-events: none; max-width: 340px; }
  .legend-row { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 14px; }
  .legend-title { color: #b6c6d4; font-weight: 700; }
  .map-legend span { display: flex; align-items: center; gap: 6px; white-space: nowrap; }
  .map-legend i { flex: none; width: 20px; border-radius: 999px; }
  .map-legend .swatch { height: 3px; }
  /* Swatches mirror the real canvas colours and dash patterns exactly. */
  .map-legend .code { background: #4aa8c9; }
  .map-legend .indirect { background: repeating-linear-gradient(90deg, #9b7fc4 0 7px, transparent 7px 12px); }
  .map-legend .bridge { background: repeating-linear-gradient(90deg, #d4923c 0 2px, transparent 2px 7px); }
  .map-legend .category { background: #41647a; }
  .legend-label { color: #6f8194; }
  .legend-foot { color: #6f8194; }
  /* The gauge swatches mirror the three real stroke widths on the canvas. */
  .map-legend .gauge { background: #8ba0b2; }
  .map-legend .weak { height: 2px; }
  .map-legend .medium { height: 3px; }
  .map-legend .strong { height: 4px; }
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
  /* An edge says four things: which way it points (arrowhead), how strongly the
     two files are coupled (width), how it relates to the selected file (colour),
     and whether the link is direct or collapsed (solid vs dashed). */
  :global(.svelte-flow__edge-path) {
    stroke: var(--edge-color, #6d879d);
    stroke-width: var(--edge-width, 1.6);
    stroke-linecap: round;
    stroke-linejoin: round;
    transition: stroke 160ms ease, stroke-width 160ms ease, opacity 160ms ease;
  }

  /* Width = how often the two files are committed together. */
  :global(.svelte-flow__edge.strength-weak) { --edge-width: 1.6; }
  :global(.svelte-flow__edge.strength-medium) { --edge-width: 2.6; }
  :global(.svelte-flow__edge.strength-strong) { --edge-width: 4; }

  /* Colour = what kind of connection this is. Fixed, so every line reads on its
     own with nothing selected. */
  :global(.svelte-flow__edge.code-edge) { --edge-color: #4aa8c9; }
  :global(.svelte-flow__edge.indirect-edge) { --edge-color: #9b7fc4; }
  :global(.svelte-flow__edge.bridge-edge) { --edge-color: #d4923c; --edge-width: 2.4; }
  :global(.svelte-flow__edge.category-edge) { --edge-color: #41647a; --edge-width: 1.4; }

  /* Dash pattern = the link is not a direct reference. */
  :global(.svelte-flow__edge.indirect-edge .svelte-flow__edge-path) { stroke-dasharray: 7 9; }
  :global(.svelte-flow__edge.bridge-edge .svelte-flow__edge-path) { stroke-dasharray: 2 7; }

  /* Selection only changes emphasis, never colour: the file's own links glow,
     the rest recede. Direction stays readable from the arrowheads. */
  :global(.svelte-flow__edge.rel-neutral) { opacity: 0.82; }
  :global(.svelte-flow__edge.rel-unrelated) { opacity: 0.14; }
  :global(.svelte-flow__edge.rel-outgoing),
  :global(.svelte-flow__edge.rel-incoming) { opacity: 1; }
  :global(.svelte-flow__edge.rel-outgoing .svelte-flow__edge-path),
  :global(.svelte-flow__edge.rel-incoming .svelte-flow__edge-path) {
    stroke-width: calc(var(--edge-width, 1.6) + 1px);
    filter: drop-shadow(0 0 7px color-mix(in srgb, var(--edge-color) 55%, transparent));
  }
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
