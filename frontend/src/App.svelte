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
  let loading = true;
  let connected = false;
  let notice = '';

  $: selected = graph?.nodes.find((node) => node.id === selectedId);
  $: changedCount = graph?.nodes.filter((node) => node.change).length ?? 0;
  $: shownRootCount = nodes.filter((node) => node.data.isRoot).length;

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

  function layoutNodes(apiNodes, apiEdges) {
    const previousPositions = new Map(nodes.map((node) => [node.id, node.position]));
    const incoming = new Map(apiNodes.map((node) => [node.id, 0]));
    const outgoing = new Map(apiNodes.map((node) => [node.id, []]));

    for (const edge of apiEdges) {
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
    const horizontalGap = 250;
    const verticalGap = 150;
    for (const [depth, level] of [...nodesByDepth.entries()].sort(([left], [right]) => left - right)) {
      level.forEach((node, index) => {
        result.push({
          id: node.id,
          type: 'code',
          data: node,
          position: previousPositions.get(node.id) ?? {
            x: (index - (level.length - 1) / 2) * horizontalGap,
            y: depth * verticalGap
          },
          ariaLabel: `${node.label}: ${node.description}`
        });
      });
    }
    return result;
  }

  async function loadGraph() {
    try {
      const response = await fetch('/api/graph', { cache: 'no-store' });
      if (!response.ok) throw new Error(`Graph request failed (${response.status})`);
      const nextGraph = await response.json();
      const review = reviewGraph(nextGraph.nodes, nextGraph.edges);
      graph = nextGraph;
      nodes = layoutNodes(review.nodes, review.edges);
      edges = review.edges.map((edge) => ({
        id: `${edge.from}:${edge.to}:${edge.kind}`,
        source: edge.from,
        target: edge.to,
        type: 'smoothstep',
        label: edge.kind === 'indirect' ? undefined : edge.kind,
        class: edge.kind === 'indirect' ? 'indirect-edge' : 'code-edge'
      }));
      if (!review.nodes.some((node) => node.id === selectedId)) {
        selectedId = review.nodes.find((node) => node.change)?.id ?? review.nodes[0]?.id ?? '';
      }
      loading = false;
      notice = '';
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
    if ('detail' in event && event.detail >= 2) openNode(node.id);
  }

  onMount(() => {
    loadGraph();
    const events = new EventSource('/events');
    events.addEventListener('ready', () => connected = true);
    events.addEventListener('graph-changed', () => {
      connected = true;
      loadGraph();
    });
    events.addEventListener('analysis-error', (event) => {
      const payload = JSON.parse(event.data);
      notice = `Keeping the last valid map: ${payload.error}`;
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
    </div>
    <div class="sync" class:online={connected}>
      <span></span>{connected ? 'live' : 'reconnecting'}
    </div>
  </header>

  <section class="summary">
    <div>
      <p class="eyebrow">Working tree now</p>
      <h1>{graph?.clean ? 'No working-tree changes' : `${changedCount} files changing`}</h1>
    </div>
    <div class="metrics" aria-label="Map summary">
      <span><strong>{shownRootCount}</strong> roots</span>
      <span><strong>{changedCount}</strong> changes</span>
      <span><strong>{nodes.length}</strong> shown</span>
    </div>
  </section>

  {#if notice}<div class="notice" role="status">{notice}</div>{/if}

  <div class="workspace">
    <section class="map" aria-label="Repository dependency map">
      {#if loading}
        <div class="loading">Building the repository map…</div>
      {:else if nodes.length === 0}
        <div class="loading">No supported source files found.</div>
      {:else}
        <SvelteFlow
          bind:nodes
          bind:edges
          {nodeTypes}
          fitView
          minZoom={0.2}
          maxZoom={2}
          onnodeclick={handleNodeClick}
          nodesConnectable={false}
          deleteKey={null}
          proOptions={{ hideAttribution: true }}
        >
          <Background variant={BackgroundVariant.Dots} gap={20} size={1.1} color="#3c3f46" />
          <Controls position="bottom-left" />
        </SvelteFlow>
      {/if}
    </section>

    <aside>
      {#if selected}
        <p class="eyebrow">Selected file</p>
        <h2>{selected.label}</h2>
        <code>{selected.id}</code>
        <h3>{selected.change ? 'Change summary' : 'File description'}</h3>
        <p class="description">{selected.description}</p>

        {#if selected.change}
          <div class="change-summary status-{selected.change.status.toLowerCase()}">
            <strong>{selected.change.status}</strong>
            <span>+{selected.change.additions} / −{selected.change.deletions}</span>
            <span>first change: line {selected.change.firstChangedLine}</span>
          </div>
          <h3>Touched symbols</h3>
          {#if selected.change.touchedSymbols.length}
            <div class="pills">
              {#each selected.change.touchedSymbols as symbol}<span>{symbol}</span>{/each}
            </div>
          {:else}
            <p class="muted">No containing Go symbol detected.</p>
          {/if}
        {:else if selected.affected}
          <p class="affected-note">Connected to a file that is changing now.</p>
        {:else if selected.isRoot}
          <p class="muted">Repository root shown even though it is unchanged.</p>
        {:else}
          <p class="muted">Unchanged in the working tree.</p>
        {/if}

        <button disabled={!selected.openable} onclick={() => openNode(selected.id)}>Open in VS Code</button>
        <p class="hint">Double-click a card to open it directly.</p>
      {:else}
        <p class="muted">Select a card to inspect it.</p>
      {/if}

      {#if graph?.otherChanges.length}
        <div class="other-changes">
          <h3>Other changes</h3>
          {#each graph.otherChanges as change}
            <button class="other-row" disabled={!change.openable} onclick={() => openNode(change.id)}>
              <span class="status-badge">{change.status}</span><span>{change.id}</span>
            </button>
          {/each}
        </div>
      {/if}
    </aside>
  </div>
</main>

<style>
  :global(*) { box-sizing: border-box; }
  :global(html) { background: #071018; }
  :global(body) { margin: 0; min-width: 320px; min-height: 100vh; color: #e5eef5; font-family: Inter, ui-sans-serif, system-ui, sans-serif; background: #071018; }
  :global(button), :global(input) { font: inherit; }
  main { min-height: 100vh; display: flex; flex-direction: column; }
  header { height: 58px; padding: 0 22px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #1b2d39; background: #08111a; }
  .brand, .sync, .metrics { display: flex; align-items: center; }
  .brand { gap: 10px; }
  .brand > span:last-child { color: #7791a4; font: 0.82rem ui-monospace, Consolas, monospace; }
  .mark { width: 11px; height: 11px; border-radius: 3px; background: #ff9f43; }
  .sync { gap: 7px; color: #7f94a4; font-size: 0.78rem; }
  .sync > span { width: 7px; height: 7px; border-radius: 50%; background: #f59e0b; }
  .sync.online > span { background: #34d399; box-shadow: 0 0 12px rgba(52, 211, 153, 0.65); }
  .summary { min-height: 92px; padding: 16px 22px; display: flex; align-items: center; justify-content: space-between; gap: 24px; border-bottom: 1px solid #1b2d39; }
  .eyebrow { margin: 0 0 5px; color: #5bbde8; font: 700 0.68rem ui-monospace, Consolas, monospace; letter-spacing: 0.14em; text-transform: uppercase; }
  h1, h2 { margin: 0; letter-spacing: -0.025em; }
  h1 { font-size: clamp(1.25rem, 2.5vw, 1.8rem); }
  h2 { margin-bottom: 7px; overflow-wrap: anywhere; }
  h3 { margin: 24px 0 10px; color: #8ba3b5; font-size: 0.72rem; letter-spacing: 0.1em; text-transform: uppercase; }
  .metrics { gap: 20px; color: #7890a1; font-size: 0.78rem; }
  .metrics span { display: grid; gap: 2px; }
  .metrics strong { color: #eaf3f8; font-size: 1.15rem; }
  .notice { padding: 9px 22px; border-bottom: 1px solid #47351b; color: #f6c56e; background: #211a10; font-size: 0.8rem; }
  .workspace { min-height: 0; flex: 1; display: grid; grid-template-columns: minmax(0, 1fr) 350px; }
  .map { position: relative; min-height: 600px; border-right: 1px solid #1b2d39; background: #17191d; }
  .loading { position: absolute; inset: 0; display: grid; place-items: center; color: #708898; }
  aside { min-width: 0; overflow-y: auto; padding: 24px; background: #09131c; }
  aside code { display: block; color: #5bbde8; overflow-wrap: anywhere; font-size: 0.78rem; }
  .description { margin: 20px 0; color: #b4c4cf; line-height: 1.6; }
  .change-summary { margin: 18px 0; display: grid; grid-template-columns: auto 1fr; gap: 5px 12px; padding: 13px; border: 1px solid #5d461c; border-radius: 10px; background: #18160f; color: #b8a77c; font-size: 0.78rem; }
  .change-summary strong { grid-row: span 2; align-self: center; color: #fbbf24; font-size: 1.15rem; }
  .change-summary.status-a { border-color: #1f5a42; background: #0e1d18; }
  .change-summary.status-a strong { color: #4ade80; }
  .change-summary.status-d { border-color: #61303c; background: #211117; }
  .change-summary.status-d strong { color: #fb7185; }
  .pills { display: flex; flex-wrap: wrap; gap: 7px; }
  .pills span, .status-badge { padding: 4px 7px; border: 1px solid #294357; border-radius: 6px; color: #9fc6dc; background: #0c1b26; font: 0.72rem ui-monospace, Consolas, monospace; }
  .muted, .hint { color: #6f8798; font-size: 0.8rem; line-height: 1.5; }
  .affected-note { padding-left: 10px; border-left: 2px solid #38bdf8; color: #9ccbe1; font-size: 0.82rem; }
  aside > button { width: 100%; margin-top: 22px; border: 0; border-radius: 8px; padding: 10px 14px; color: #061018; background: #67e8f9; font-weight: 750; cursor: pointer; }
  button:disabled { opacity: 0.45; cursor: not-allowed; }
  .hint { margin-top: 8px; text-align: center; }
  .other-changes { margin-top: 32px; padding-top: 6px; border-top: 1px solid #1d303d; }
  .other-row { width: 100%; display: grid; grid-template-columns: auto 1fr; align-items: center; gap: 9px; border: 0; padding: 8px 0; color: #aabcc8; background: none; text-align: left; font-size: 0.76rem; cursor: pointer; overflow-wrap: anywhere; }
  :global(.svelte-flow) { background: #17191d; }
  :global(.svelte-flow__edge-path) { stroke: #5b5f67; stroke-width: 1.5; }
  :global(.svelte-flow__edge.indirect-edge .svelte-flow__edge-path) { stroke: #60677d; stroke-dasharray: 4 5; }
  :global(.svelte-flow__edge-text) { fill: #628398; font-size: 9px; }
  :global(.svelte-flow__edge-textbg) { fill: #071018; }
  :global(.svelte-flow__controls) { border: 1px solid #264050; border-radius: 8px; overflow: hidden; box-shadow: none; }
  :global(.svelte-flow__controls-button) { border: 0; border-bottom: 1px solid #264050; color: #a7bfce; background: #10202b; }
  :global(.svelte-flow__controls-button:hover) { background: #183040; }
  @media (max-width: 850px) {
    .summary { align-items: flex-start; flex-direction: column; }
    .metrics { width: 100%; justify-content: space-between; }
    .workspace { grid-template-columns: 1fr; }
    .map { min-height: 65vh; border-right: 0; border-bottom: 1px solid #1b2d39; }
  }
</style>
