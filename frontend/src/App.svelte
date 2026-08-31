<script>
  import { onMount } from 'svelte';
  import { Background, BackgroundVariant, Position, SvelteFlow } from '@xyflow/svelte';
  import '@xyflow/svelte/dist/style.css';
  import CodeCard from './CodeCard.svelte';
  import Jump from './Jump.svelte';
  import Popover from './Popover.svelte';
  import Scrubber from './Scrubber.svelte';
  import Segmented from './Segmented.svelte';
  import Sheet from './Sheet.svelte';
  import { SPRINGS, Spring, clamp, prefersReducedMotion } from './lib/motion.js';

  const nodeTypes = { code: CodeCard };

  // Half the handle size. Edges anchor a hair outside the card, so an arrowhead
  // lands on the border rather than under it.
  const HANDLE = 3;

  const MIN_ZOOM = 0.3;
  const MAX_ZOOM = 1.8;
  const PANEL_WIDTH = 428;
  const SHEET_PEEK = 300;

  const viewOptions = [
    { id: 'changes', label: 'Changing', hint: 'What is moving right now, and what it touches' },
    { id: 'calls', label: 'Calls', hint: 'Every source file and what it uses' },
    { id: 'both', label: 'Everything', hint: 'The whole graph, with changes highlighted' }
  ];
  const scopeOptions = [
    { id: 'all', label: 'Whole repository' },
    { id: 'frontend', label: 'Frontend only' },
    { id: 'backend', label: 'Backend only' }
  ];

  let graph = $state(null);
  let nodes = $state.raw([]);
  let edges = $state.raw([]);
  let viewport = $state({ x: 0, y: 0, zoom: 1 });

  let selectedId = $state('');
  let hoverId = $state('');
  let view = $state('changes');
  let scope = $state('all');
  let period = $state('30');
  let loading = $state(true);
  let connected = $state(false);
  let notice = $state('');
  let activityLog = $state([]);

  let paletteOpen = $state(false);
  let scopeOpen = $state(false);
  let helpOpen = $state(false);
  let statusOpen = $state(false);

  let narrow = $state(false);
  let mapEl = $state(null);
  let sheetExtent = $state(0);

  // Deliberately outside the reactive graph: bookkeeping for the gesture and
  // animation layers, where re-rendering on every frame would defeat the point.
  let draggingId = null;
  let pinnedPositions = new Map();
  let layoutTargets = new Map();
  const motion = new Map();
  let pumping = false;
  let lastExtent = 0;
  let visible = { nodes: [], edges: [] };
  let neighborIds = new Set();
  let commitSets = new Map();

  const selected = $derived(
    selectedId ? (graph?.nodes.find((node) => node.id === selectedId) ?? null) : null
  );
  // Hovering previews the relationship a click commits to, so the map answers
  // before you have to ask it.
  const focusId = $derived(hoverId || selectedId);
  const changedCount = $derived(
    (graph?.nodes.filter((node) => node.change).length ?? 0) + (graph?.otherChanges?.length ?? 0)
  );
  const sourceCount = $derived(scopedSourceNodes(graph?.nodes ?? []).length);
  const scopeLabel = $derived(
    scopeOptions.find((option) => option.id === scope)?.label ?? 'Whole repository'
  );
  const selectedCommits = $derived(activityCount(selected?.activity));
  const dependsOn = $derived(relatedNodes('out'));
  const usedBy = $derived(relatedNodes('in'));
  const partners = $derived([...dependsOn, ...usedBy].slice(0, 5));
  const detents = $derived(
    narrow ? [SHEET_PEEK, Math.round(Math.min(720, window.innerHeight * 0.86))] : [PANEL_WIDTH]
  );

  // --- Graph shaping ------------------------------------------------------
  // Which files belong on screen for the current view and scope. This is the
  // honest reading of the repository; the redesign has no business rewriting it.

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
    return {
      nodes: apiNodes.filter((node) => visibleIds.has(node.id)),
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

    return result.sort(
      (left, right) =>
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
    const childCount = scopedSourceNodes(graph?.nodes ?? []).filter(
      (node) => nodeArea(node) === category
    ).length;
    return {
      id: frontend ? '__frontend__' : '__backend__',
      label: frontend ? 'Frontend' : 'Backend',
      language: 'group',
      kind: 'folder',
      description: frontend
        ? `${childCount} client-side source files and UI entry points.`
        : `${childCount} server-side source files and analysis logic.`,
      isRoot: true,
      openable: false,
      area: category,
      activity: { commits30: 0, commits90: 0, commitsAll: 0, people: 0, recentCommits: [] }
    };
  }

  function categoryRoots(apiNodes, apiEdges, category) {
    const categoryNodes = apiNodes.filter((node) => nodeArea(node) === category);
    const categoryIds = new Set(categoryNodes.map((node) => node.id));
    const incoming = new Set(
      apiEdges
        .filter((edge) => categoryIds.has(edge.from) && categoryIds.has(edge.to))
        .map((edge) => edge.to)
    );
    const roots = categoryNodes.filter((node) => node.isRoot || !incoming.has(node.id));
    return (roots.length ? roots : categoryNodes.slice(0, 4)).slice(0, 6);
  }

  // --- Layout -------------------------------------------------------------
  // Columns by dependency depth, each column stacked by measured card height so
  // two cards can never land on top of each other.

  const CARD_WIDTH = 234;
  const SCOPE_CARD_WIDTH = 250;
  const COLUMN_GAP = 104;
  const ROW_GAP = 34;
  const MAX_COLUMN_HEIGHT = 1500;
  const CARD_BASE_HEIGHT = 118;
  const TITLE_LINE_HEIGHT = 19;
  const SAFETY_PAD = 10;

  function isScopeId(id) {
    return String(id ?? '').startsWith('__');
  }

  function cardWidth(node) {
    return isScopeId(node.id) ? SCOPE_CARD_WIDTH : CARD_WIDTH;
  }

  // Mirrors CodeCard: header, title, two clamped description lines and the
  // activity rail. Titles wrap at roughly 26 characters inside a 234px card.
  // This estimate must never come in under the real height or cards overlap.
  function cardHeight(node) {
    if (isScopeId(node.id)) return 118;
    const titleLines = Math.min(2, Math.ceil(String(node.label ?? '').length / 26) || 1);
    return CARD_BASE_HEIGHT + (titleLines - 1) * TITLE_LINE_HEIGHT + SAFETY_PAD;
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
      roots = apiNodes
        .filter((node) => (incoming.get(node.id) ?? []).length === 0)
        .map((node) => node.id);
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

    // Anything the walk never reached still needs a home; give it a column past
    // the deepest reachable one rather than dropping it at zero.
    const orphanDepth = Math.max(0, ...depths.values()) + 1;
    for (const node of apiNodes) {
      if (!depths.has(node.id)) depths.set(node.id, orphanDepth);
    }
    return depths;
  }

  // Order each column by the average row of the parents already placed to its
  // left, so children sit next to their parents and most crossings disappear.
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

  function computeLayout(apiNodes, apiEdges) {
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
          strip.reduce((sum, node) => sum + cardHeight(node), 0) +
          Math.max(0, strip.length - 1) * ROW_GAP;
        const stripWidth = Math.max(...strip.map(cardWidth));

        let y = -stripHeight / 2;
        for (const node of strip) {
          positions.set(node.id, { x: x + (stripWidth - cardWidth(node)) / 2, y });
          placedRows.set(node.id, y + cardHeight(node) / 2);
          y += cardHeight(node) + ROW_GAP;
        }
        x += stripWidth + COLUMN_GAP;
      }
    }

    return positions;
  }

  // --- Connection strength -------------------------------------------------
  // How often two files were committed together: the number edge thickness
  // encodes, and the only honest answer to "how connected are these".

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

  function strengthTier(count) {
    if (count >= 5) return 'strong';
    if (count >= 2) return 'medium';
    return 'weak';
  }

  function edgeKindKey(kind) {
    if (kind === 'indirect') return 'indirect';
    if (kind === 'bridge') return 'bridge';
    if (kind === 'category') return 'group';
    return 'code';
  }

  // Colour carries the kind of connection and never changes with selection, so
  // a line means something on its own before you click anything.
  const KIND_COLORS = {
    code: 'var(--calls)',
    indirect: 'var(--indirect)',
    bridge: 'var(--bridge)',
    group: 'var(--group)'
  };

  const RELATION_PAINT_ORDER = { unrelated: 0, neutral: 1, incoming: 2, outgoing: 3 };

  // --- Render --------------------------------------------------------------

  function activityCount(activity = null) {
    if (!activity) return 0;
    if (period === '90') return activity.commits90 ?? 0;
    if (period === 'all') return activity.commitsAll ?? 0;
    return activity.commits30 ?? 0;
  }

  function renderGraph(nextGraph = graph) {
    if (!nextGraph) return;
    visible = visibleGraph(nextGraph.nodes, nextGraph.edges);
    if (!visible.nodes.some((node) => node.id === selectedId)) selectedId = '';
    rebuildCommitSets(nextGraph.nodes);
    retargetLayout(computeLayout(visible.nodes, visible.edges));
    syncNodes();
    syncEdges();
  }

  /**
   * Move cards to their new homes rather than teleporting them. Each axis gets
   * its own spring, so a card whose column changed but whose row did not
   * travels horizontally without a wobble on the other axis.
   */
  function retargetLayout(positions) {
    layoutTargets = positions;
    const alive = new Set(visible.nodes.map((node) => node.id));
    for (const id of [...motion.keys()]) if (!alive.has(id)) motion.delete(id);

    for (const node of visible.nodes) {
      const target = pinnedPositions.get(node.id) ?? positions.get(node.id) ?? { x: 0, y: 0 };
      const existing = motion.get(node.id);
      if (!existing) {
        // A card appearing for the first time belongs where it belongs; only
        // cards already on screen have a journey to make.
        motion.set(node.id, { x: new Spring(target.x), y: new Spring(target.y) });
        continue;
      }
      existing.x.to(target.x, SPRINGS.move);
      existing.y.to(target.y, SPRINGS.move);
    }
    startPump();
  }

  // The springs step on the motion module's clock; this loop only reads them
  // and republishes the node array, then stops as soon as everything settles.
  function startPump() {
    if (prefersReducedMotion()) {
      syncNodes();
      return;
    }
    if (pumping) return;
    pumping = true;
    requestAnimationFrame(function frame() {
      const moving = [...motion.values()].some((entry) => entry.x.moving || entry.y.moving);
      syncNodes();
      if (moving) requestAnimationFrame(frame);
      else pumping = false;
    });
  }

  function relationFor(id) {
    if (!focusId) return 'none';
    if (id === focusId) return 'self';
    return neighborIds.has(id) ? 'linked' : 'faded';
  }

  function syncNodes() {
    if (draggingId) return;

    neighborIds = new Set();
    if (focusId) {
      for (const edge of visible.edges) {
        if (edge.from === focusId) neighborIds.add(edge.to);
        if (edge.to === focusId) neighborIds.add(edge.from);
      }
    }

    const counts = visible.nodes.map((node) => activityCount(node.activity));
    const peak = Math.max(1, ...counts);

    nodes = visible.nodes.map((node, index) => {
      const spot = motion.get(node.id);
      const size = { width: cardWidth(node), height: cardHeight(node) };
      return {
        id: node.id,
        type: 'code',
        ...size,
        measured: size,
        // Declared rather than measured. Republishing this array hands the flow
        // brand new node objects, and anything it had *measured* about the old
        // ones -- including where the edges attach -- is discarded, which
        // silently drops every line on the map. Cards are rendered at a size we
        // computed ourselves, so stating the anchors outright is both accurate
        // and stable across every re-render.
        handles: [
          {
            id: null,
            type: 'target',
            position: Position.Top,
            x: size.width / 2 - HANDLE,
            y: -HANDLE,
            width: HANDLE * 2,
            height: HANDLE * 2
          },
          {
            id: null,
            type: 'source',
            position: Position.Bottom,
            x: size.width / 2 - HANDLE,
            y: size.height - HANDLE,
            width: HANDLE * 2,
            height: HANDLE * 2
          }
        ],
        selected: node.id === selectedId,
        position: {
          x: spot?.x.value ?? layoutTargets.get(node.id)?.x ?? 0,
          y: spot?.y.value ?? layoutTargets.get(node.id)?.y ?? 0
        },
        data: {
          ...node,
          periodCommits: counts[index],
          intensity: counts[index] / peak,
          relation: relationFor(node.id)
        },
        ariaLabel: `${node.label}. ${node.description}`
      };
    });
  }

  function syncEdges() {
    edges = visible.edges
      .map((edge) => {
        const relation = !focusId
          ? 'neutral'
          : edge.from === focusId
            ? 'outgoing'
            : edge.to === focusId
              ? 'incoming'
              : 'unrelated';
        const structural = edge.kind === 'category' || edge.kind === 'bridge';
        const coChange = structural ? 0 : coChangeCount(edge.from, edge.to);
        const kindKey = edgeKindKey(edge.kind);
        return {
          id: `${edge.from}:${edge.to}:${edge.kind}`,
          source: edge.from,
          target: edge.to,
          type: 'default',
          interactionWidth: 22,
          // Structural lines group the map; they are not dependencies, so they
          // get no arrowhead and no strength.
          markerEnd: structural
            ? undefined
            : { type: 'arrowclosed', width: 12, height: 12, color: KIND_COLORS[kindKey] },
          class: `${kindKey}-edge strength-${strengthTier(coChange)} rel-${relation}`,
          data: { coChange, relation, kind: edge.kind },
          relation
        };
      })
      // Dimmed lines first, the focused file's lines last, so the highlighted
      // path is never buried under the rest of the graph.
      .sort((left, right) => RELATION_PAINT_ORDER[left.relation] - RELATION_PAINT_ORDER[right.relation]);
  }

  // Focus and the active window change what a card says about itself, so cards
  // are rebuilt for both -- but never the layout, which stays put.
  $effect(() => {
    focusId;
    period;
    if (graph) {
      syncNodes();
      syncEdges();
    }
  });

  // --- Camera --------------------------------------------------------------

  function syncCamera() {
    viewport = { x: camera.x.value, y: camera.y.value, zoom: camera.zoom.value };
  }

  const camera = {
    x: new Spring(0, { ...SPRINGS.move, onchange: syncCamera }),
    y: new Spring(0, { ...SPRINGS.move, onchange: syncCamera }),
    zoom: new Spring(1, { ...SPRINGS.move, epsilon: 0.0006, onchange: syncCamera })
  };

  /** Hand the camera back to the user. Any touch of the map wins immediately. */
  function releaseCamera() {
    camera.x.hold(viewport.x);
    camera.y.hold(viewport.y);
    camera.zoom.hold(viewport.zoom);
  }

  function occlusion() {
    if (!selectedId) return { right: 0, bottom: 0 };
    return narrow ? { right: 0, bottom: detents[0] } : { right: PANEL_WIDTH, bottom: 0 };
  }

  function boundsOf(ids) {
    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    for (const id of ids) {
      const node = visible.nodes.find((entry) => entry.id === id);
      const spot = pinnedPositions.get(id) ?? layoutTargets.get(id);
      if (!node || !spot) continue;
      minX = Math.min(minX, spot.x);
      minY = Math.min(minY, spot.y);
      maxX = Math.max(maxX, spot.x + cardWidth(node));
      maxY = Math.max(maxY, spot.y + cardHeight(node));
    }
    if (minX === Infinity) return null;
    return { x: minX, y: minY, width: maxX - minX, height: maxY - minY };
  }

  function flyTo(ids, { padding = 90, maxZoom = 1 } = {}) {
    const bounds = boundsOf(ids);
    const rect = mapEl?.getBoundingClientRect();
    if (!bounds || !rect) return;

    // Aim at the part of the map the inspector is not covering, so arriving
    // somewhere never puts it underneath the panel.
    const hidden = occlusion();
    const width = Math.max(160, rect.width - hidden.right);
    const height = Math.max(160, rect.height - hidden.bottom);
    const zoom = clamp(
      Math.min(
        (width - padding * 2) / Math.max(1, bounds.width),
        (height - padding * 2) / Math.max(1, bounds.height)
      ),
      MIN_ZOOM,
      maxZoom
    );
    const centerX = bounds.x + bounds.width / 2;
    const centerY = bounds.y + bounds.height / 2;

    // Start from where the camera actually is, which may be somewhere the user
    // panned to rather than where the last flight ended.
    camera.x.hold(viewport.x);
    camera.y.hold(viewport.y);
    camera.zoom.hold(viewport.zoom);
    camera.x.to(width / 2 - centerX * zoom, SPRINGS.move);
    camera.y.to(height / 2 - centerY * zoom, SPRINGS.move);
    camera.zoom.to(zoom, SPRINGS.move);
  }

  function fitAll() {
    flyTo(
      visible.nodes.map((node) => node.id),
      { padding: 70, maxZoom: 0.95 }
    );
  }

  function focusOn(id) {
    // Fly to the file *and its immediate links*, so arriving somewhere never
    // costs the context you were using to get there.
    const withNeighbors = new Set([id]);
    for (const edge of visible.edges) {
      if (edge.from === id) withNeighbors.add(edge.to);
      if (edge.to === id) withNeighbors.add(edge.from);
    }
    flyTo([...withNeighbors], { padding: 80, maxZoom: 1 });
  }

  function zoomBy(factor) {
    const rect = mapEl?.getBoundingClientRect();
    if (!rect) return;
    const hidden = occlusion();
    const anchorX = (rect.width - hidden.right) / 2;
    const anchorY = (rect.height - hidden.bottom) / 2;
    const next = clamp(viewport.zoom * factor, MIN_ZOOM, MAX_ZOOM);
    const ratio = next / viewport.zoom;
    releaseCamera();
    camera.zoom.to(next, SPRINGS.ui);
    camera.x.to(anchorX - (anchorX - viewport.x) * ratio, SPRINGS.ui);
    camera.y.to(anchorY - (anchorY - viewport.y) * ratio, SPRINGS.ui);
  }

  /**
   * As the panel grows it takes screen from the right; slide the map the other
   * way by half of that, so the card you were reading stays where you left it.
   */
  function onSheetExtent(value) {
    sheetExtent = value;
    const delta = value - lastExtent;
    lastExtent = value;
    if (!delta || camera.x.moving || camera.y.moving) return;
    if (narrow) camera.y.hold(viewport.y - delta / 2);
    else camera.x.hold(viewport.x - delta / 2);
  }

  // --- Selection -----------------------------------------------------------

  function select(id, { fly = true } = {}) {
    if (!id) return;
    const opening = !selectedId;
    selectedId = id;
    if (opening) lastExtent = 0;
    // Wait a frame so `occlusion()` already knows the panel is coming; the
    // flight is then computed against the space that will actually be left.
    if (fly) requestAnimationFrame(() => focusOn(id));
  }

  function clearSelection() {
    selectedId = '';
  }

  // --- Data ----------------------------------------------------------------

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
    if (hours < 24) return `${hours} h ago`;
    const days = Math.floor(hours / 24);
    if (days < 30) return `${days} days ago`;
    const months = Math.floor(days / 30);
    if (months < 12) return `${months} months ago`;
    return `${Math.floor(months / 12)} years ago`;
  }

  function partnerWidth(node) {
    const max = Math.max(1, selectedCommits, ...partners.map((item) => activityCount(item.activity)));
    return `${Math.max(10, Math.round((activityCount(node.activity) / max) * 100))}%`;
  }

  function lead() {
    if (!selected) return '';
    return selected.change ? selected.change.summary.changed : selected.description;
  }

  function addActivity(message) {
    activityLog = [{ at: new Date().toLocaleTimeString(), message }, ...activityLog].slice(0, 10);
  }

  async function loadGraph() {
    try {
      const response = await fetch('/api/graph', { cache: 'no-store' });
      if (!response.ok) throw new Error(`Graph request failed (${response.status})`);
      const nextGraph = await response.json();
      const previousRevision = graph?.revision;
      const first = !graph;
      graph = nextGraph;
      renderGraph(nextGraph);
      loading = false;
      notice = '';
      // Two frames: one for the flow to mount, one for it to have a size.
      if (first) requestAnimationFrame(() => requestAnimationFrame(fitAll));
      if (previousRevision !== nextGraph.revision) {
        addActivity(`Map rebuilt at revision ${nextGraph.revision}`);
      }
    } catch (error) {
      loading = false;
      notice = error instanceof Error ? error.message : String(error);
    }
  }

  async function openInEditor(id) {
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

  function setView(next) {
    view = next;
    // A different graph gets a fresh layout; keeping hand-dragged spots from the
    // previous one is what drops new cards on top of old ones.
    pinnedPositions = new Map();
    renderGraph();
    requestAnimationFrame(() => (selectedId ? focusOn(selectedId) : fitAll()));
  }

  function setScope(next) {
    scope = next;
    scopeOpen = false;
    selectedId = '';
    pinnedPositions = new Map();
    renderGraph();
    requestAnimationFrame(fitAll);
  }

  function showChanges() {
    const ids = visible.nodes.filter((node) => node.change).map((node) => node.id);
    if (ids.length) flyTo(ids, { padding: 80, maxZoom: 1 });
    else fitAll();
  }

  function onNodeClick({ node, event }) {
    select(node.id);
    if ('detail' in event && event.detail >= 2) openInEditor(node.id);
  }

  function onNodeDragStop({ targetNode }) {
    if (targetNode) {
      pinnedPositions.set(targetNode.id, { ...targetNode.position });
      const spot = motion.get(targetNode.id);
      spot?.x.jump(targetNode.position.x);
      spot?.y.jump(targetNode.position.y);
    }
    draggingId = null;
    syncNodes();
  }

  function onKeyDown(event) {
    const typing = ['INPUT', 'TEXTAREA'].includes(event.target?.tagName);
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      paletteOpen = !paletteOpen;
      return;
    }
    if (typing) return;
    if (event.key === 'Escape') {
      // Never trap: escape unwinds one layer at a time, ending at the bare map.
      if (paletteOpen) paletteOpen = false;
      else if (scopeOpen || helpOpen || statusOpen) scopeOpen = helpOpen = statusOpen = false;
      else clearSelection();
    } else if (event.key === 'f') {
      fitAll();
    } else if (event.key === '/') {
      event.preventDefault();
      paletteOpen = true;
    }
  }

  $effect(() => {
    if (!notice) return;
    const timer = setTimeout(() => (notice = ''), 4200);
    return () => clearTimeout(timer);
  });

  // Any touch of the map takes the camera back mid-flight, which is the whole
  // point: a flight in progress is never a lockout.
  $effect(() => {
    if (!mapEl) return;
    const stop = () => releaseCamera();
    mapEl.addEventListener('pointerdown', stop, true);
    mapEl.addEventListener('wheel', stop, { capture: true, passive: true });
    return () => {
      mapEl.removeEventListener('pointerdown', stop, true);
      mapEl.removeEventListener('wheel', stop, { capture: true });
    };
  });

  onMount(() => {
    const media = matchMedia('(max-width: 900px)');
    const syncWidth = () => (narrow = media.matches);
    syncWidth();
    media.addEventListener('change', syncWidth);

    loadGraph();
    const events = new EventSource('/events');
    events.addEventListener('ready', () => {
      connected = true;
      addActivity('Connected to the live repository watcher');
    });
    events.addEventListener('graph-changed', (event) => {
      const payload = JSON.parse(event.data);
      connected = true;
      addActivity(`Repository changed - revision ${payload.revision}`);
      loadGraph();
    });
    events.addEventListener('analysis-error', (event) => {
      const payload = JSON.parse(event.data);
      notice = `Keeping the last valid map: ${payload.error}`;
      addActivity(`Analysis error: ${payload.error}`);
    });
    events.onerror = () => (connected = false);

    return () => {
      events.close();
      media.removeEventListener('change', syncWidth);
    };
  });
</script>

<svelte:head>
  <meta name="description" content="A live, local map of the current Git repository" />
  <title>{graph?.repository ?? 'gitcs'} · live map</title>
</svelte:head>

<svelte:window onkeydown={onKeyDown} />

<main>
  <div class="map" bind:this={mapEl}>
    {#if loading}
      <div class="veil"><p class="t-body">Reading the repository…</p></div>
    {:else if nodes.length === 0}
      <div class="veil"><p class="t-body">No supported source files found here.</p></div>
    {:else}
      <SvelteFlow
        bind:nodes
        bind:edges
        bind:viewport
        {nodeTypes}
        minZoom={MIN_ZOOM}
        maxZoom={MAX_ZOOM}
        colorMode="system"
        nodeDragThreshold={4}
        paneClickDistance={4}
        nodesConnectable={false}
        deleteKey={null}
        proOptions={{ hideAttribution: true }}
        onnodeclick={onNodeClick}
        onnodepointerenter={({ node }) => (hoverId = node.id)}
        onnodepointerleave={() => (hoverId = '')}
        onnodedragstart={({ targetNode }) => (draggingId = targetNode?.id ?? null)}
        onnodedragstop={onNodeDragStop}
        onpaneclick={clearSelection}
      >
        <Background variant={BackgroundVariant.Dots} gap={26} size={1} color="var(--hairline)" />
      </SvelteFlow>
    {/if}

    <!-- Chrome floats over the map instead of taking a strip away from it. -->
    <div class="chrome top">
      <div class="cluster">
        <div class="anchor">
          <button
            class="pill repo"
            data-popover-trigger
            aria-expanded={statusOpen}
            onclick={() => (statusOpen = !statusOpen)}
          >
            <span class="dot" class:live={connected}></span>
            <strong class="t-title">{graph?.repository ?? 'loading'}</strong>
            <em class="t-mono">{graph?.branch ?? 'main'}</em>
          </button>
          <Popover
            open={statusOpen}
            align="start"
            label="Repository status"
            onclose={() => (statusOpen = false)}
          >
            <p class="t-label pop-head">Status</p>
            <p class="t-small pop-line">
              {connected ? 'Watching this repository live.' : 'Not connected to the watcher.'}
              Last rebuild {graph?.generatedAt ? relativeTime(graph.generatedAt) : 'pending'}.
            </p>
            <p class="t-small pop-line">{sourceCount} source files · {changedCount} changing.</p>
            {#if graph?.otherChanges?.length}
              <p class="t-label pop-head">Changed, not on the map</p>
              <ul class="pop-list">
                {#each graph.otherChanges.slice(0, 6) as change}
                  <li class="t-mono">{change.label}</li>
                {/each}
              </ul>
            {/if}
            <p class="t-label pop-head">Recent events</p>
            <ul class="pop-list">
              {#each activityLog.slice(0, 5) as entry}
                <li class="t-small"><b>{entry.at}</b> {entry.message}</li>
              {:else}
                <li class="t-small dim">Nothing yet.</li>
              {/each}
            </ul>
          </Popover>
        </div>

        {#if changedCount > 0}
          <button class="pill accent" onclick={showChanges}>{changedCount} changing</button>
        {/if}
      </div>

      <Segmented options={viewOptions} value={view} onchange={setView} label="What the map shows" />

      <div class="cluster end">
        <div class="anchor">
          <button
            class="pill"
            data-popover-trigger
            aria-expanded={scopeOpen}
            onclick={() => (scopeOpen = !scopeOpen)}
          >
            {scopeLabel}
          </button>
          <Popover open={scopeOpen} label="Scope" onclose={() => (scopeOpen = false)}>
            <p class="t-label pop-head">Scope</p>
            {#each scopeOptions as option}
              <button
                class="pop-choice"
                class:on={scope === option.id}
                onclick={() => setScope(option.id)}
              >
                {option.label}
              </button>
            {/each}
          </Popover>
        </div>

        <button class="pill" onclick={() => (paletteOpen = true)}>Jump <kbd>⌘K</kbd></button>

        <div class="anchor">
          <button
            class="pill icon"
            data-popover-trigger
            aria-expanded={helpOpen}
            aria-label="How to read this map"
            onclick={() => (helpOpen = !helpOpen)}
          >
            ?
          </button>
          <Popover open={helpOpen} label="How to read this map" onclose={() => (helpOpen = false)}>
            <p class="t-label pop-head">Reading the map</p>
            <p class="t-small pop-line">An arrow points from a file to what it uses.</p>
            <ul class="legend">
              <li><i class="swatch code"></i><span class="t-small">uses directly</span></li>
              <li><i class="swatch indirect"></i><span class="t-small">uses via files not shown</span></li>
              <li><i class="swatch bridge"></i><span class="t-small">frontend ↔ backend</span></li>
              <li><i class="swatch group"></i><span class="t-small">grouping, not a dependency</span></li>
            </ul>
            <p class="t-label pop-head">Line weight</p>
            <ul class="legend">
              <li><i class="gauge weak"></i><span class="t-small">rarely changed together</span></li>
              <li><i class="gauge medium"></i><span class="t-small">sometimes</span></li>
              <li><i class="gauge strong"></i><span class="t-small">often</span></li>
            </ul>
            <p class="t-label pop-head">Keys</p>
            <p class="t-small pop-line">
              <kbd>⌘K</kbd> jump · <kbd>f</kbd> fit · <kbd>esc</kbd> back · double-click a card to open it
            </p>
          </Popover>
        </div>
      </div>
    </div>

    <div class="chrome bottom" class:receded={narrow && sheetExtent > 40}>
      <div class="zoom">
        <button onclick={() => zoomBy(1 / 1.25)} aria-label="Zoom out">−</button>
        <button onclick={fitAll}>Fit</button>
        <button onclick={() => zoomBy(1.25)} aria-label="Zoom in">+</button>
      </div>
      {#if graph?.activity?.length}
        <Scrubber buckets={graph.activity} {period} onperiod={(next) => (period = next)} />
      {/if}
    </div>

    {#if notice}
      <div class="toast t-small" role="status">{notice}</div>
    {/if}
  </div>

  <Sheet
    open={Boolean(selected)}
    axis={narrow ? 'y' : 'x'}
    {detents}
    label="File inspector"
    onclose={clearSelection}
    onextent={onSheetExtent}
  >
    {#snippet header()}
      {#if selected}
        <div class="ins-head">
          <div class="ins-id">
            <p class="t-label eyebrow" class:hot={Boolean(selected.change)}>
              {selected.change ? 'Changing now' : selected.kind === 'folder' ? 'Group' : 'Source file'}
            </p>
            <h2 class="t-display">{selected.label}</h2>
            <code class="t-mono">{selected.id}</code>
          </div>
          <button class="close" onclick={clearSelection} aria-label="Close the inspector">✕</button>
        </div>
        <p class="t-body lead">{lead()}</p>
        <div class="stats">
          <div>
            <strong>{selectedCommits}</strong>
            <span class="t-small">commits · {period === 'all' ? 'all time' : `${period} days`}</span>
          </div>
          <div>
            <strong>{selected.activity?.people ?? 0}</strong>
            <span class="t-small">people</span>
          </div>
          <div>
            <strong class="terse">{relativeTime(selected.activity?.lastChangedAt)}</strong>
            <span class="t-small">last change</span>
          </div>
        </div>
      {/if}
    {/snippet}

    {#snippet children()}
      {#if selected}
        <div class="ins-body">
          {#if selected.change}
            <h3 class="t-label">What changed</h3>
            <dl class="change">
              <dt class="t-small">Previously</dt>
              <dd class="t-small">{selected.change.summary.previous}</dd>
              <dt class="t-small">Now</dt>
              <dd class="t-small">{selected.change.summary.current}</dd>
              <dt class="t-small">Impact</dt>
              <dd class="t-small">{selected.change.summary.impact}</dd>
            </dl>
            <p class="t-mono diffline">
              <b>+{selected.change.additions}</b>
              <i>−{selected.change.deletions}</i>
              <span>from line {selected.change.firstChangedLine}</span>
            </p>
          {/if}

          {#if partners.length}
            <h3 class="t-label">Usually changes with</h3>
            <div class="partners">
              {#each partners as item}
                <button onclick={() => select(item.id)}>
                  <span class="t-small">{item.label}</span>
                  <em class="t-mono">{activityCount(item.activity)}</em>
                  <b><i style="width:{partnerWidth(item)}"></i></b>
                </button>
              {/each}
            </div>
          {/if}

          <h3 class="t-label">Recent commits</h3>
          <ul class="commits">
            {#each selected.activity?.recentCommits ?? [] as commit}
              <li>
                <b class="t-mono">{commit.hash}</b>
                <span class="t-small">{commit.message}</span>
                <em class="t-small">{commit.author || 'unknown'} · {relativeTime(commit.when)}</em>
              </li>
            {:else}
              <li class="t-small dim">No commits recorded for this file.</li>
            {/each}
          </ul>

          <h3 class="t-label">Depends on</h3>
          <div class="chips">
            {#each dependsOn.slice(0, 8) as item}
              <button class="t-mono" onclick={() => select(item.id)}>{item.label}</button>
            {:else}
              <span class="t-small dim">Nothing detected.</span>
            {/each}
          </div>

          <h3 class="t-label">Used by</h3>
          <div class="chips">
            {#each usedBy.slice(0, 8) as item}
              <button class="t-mono" onclick={() => select(item.id)}>{item.label}</button>
            {:else}
              <span class="t-small dim">Nothing detected.</span>
            {/each}
          </div>

          <button class="open" disabled={!selected.openable} onclick={() => openInEditor(selected.id)}>
            Open in editor
          </button>
        </div>
      {/if}
    {/snippet}
  </Sheet>

  <Jump
    open={paletteOpen}
    files={nodes.filter((node) => !node.id.startsWith('__')).map((node) => node.data)}
    onpick={(id) => {
      paletteOpen = false;
      select(id);
    }}
    onclose={() => (paletteOpen = false)}
  />
</main>

<style>
  main {
    position: relative;
    height: 100dvh;
    overflow: hidden;
    background: var(--ground);
  }

  .map {
    position: absolute;
    inset: 0;
  }

  .veil {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    color: var(--text-3);
  }

  /* --- Floating chrome --------------------------------------------------- */

  .chrome {
    position: absolute;
    left: 0;
    right: 0;
    z-index: 20;
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 14px 16px;
    /* The bar floats; the map keeps every pixel underneath it. */
    pointer-events: none;
  }

  .chrome > :global(*) {
    pointer-events: auto;
  }

  .top {
    top: 0;
    justify-content: space-between;
  }

  .bottom {
    bottom: 0;
    align-items: flex-end;
    justify-content: space-between;
    transition: opacity 200ms ease, transform 240ms cubic-bezier(0.2, 0, 0, 1);
  }

  /* When the sheet comes up on a narrow screen the bottom chrome is behind it;
     it leaves rather than sitting there being covered. */
  .bottom.receded {
    opacity: 0;
    transform: translateY(18px);
    pointer-events: none;
  }

  .cluster {
    display: flex;
    align-items: center;
    gap: 8px;
    pointer-events: auto;
  }

  .anchor {
    position: relative;
  }

  .pill {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    height: 34px;
    padding: 0 12px;
    border: 1px solid var(--hairline);
    border-radius: var(--radius-chip);
    background: var(--material);
    backdrop-filter: blur(var(--material-blur)) saturate(180%);
    box-shadow: var(--shadow-card);
    color: var(--text-2);
    font-size: 0.79rem;
    font-weight: 560;
    letter-spacing: -0.004em;
    white-space: nowrap;
    cursor: pointer;
    transition: transform 90ms ease, color 140ms ease, border-color 140ms ease;
  }

  /* Feedback on the press, not on the click that follows. */
  .pill:active {
    transform: scale(0.968);
  }

  .pill:hover {
    color: var(--text);
    border-color: var(--hairline-strong);
  }

  .pill:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .pill.icon {
    width: 34px;
    padding: 0;
    justify-content: center;
    font-weight: 700;
  }

  .pill.accent {
    color: var(--warn);
    border-color: color-mix(in srgb, var(--warn) 42%, var(--hairline));
    background: color-mix(in srgb, var(--warn) 12%, var(--material));
  }

  .repo strong {
    color: var(--text);
  }

  .repo em {
    color: var(--text-3);
    font-style: normal;
  }

  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--warn);
  }

  .dot.live {
    background: var(--live);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--live) 22%, transparent);
  }

  .zoom {
    display: flex;
    align-items: center;
    padding: 3px;
    border: 1px solid var(--hairline);
    border-radius: var(--radius-chip);
    background: var(--material);
    backdrop-filter: blur(var(--material-blur)) saturate(180%);
    box-shadow: var(--shadow-card);
  }

  .zoom button {
    min-width: 30px;
    height: 28px;
    padding: 0 9px;
    border: 0;
    border-radius: 7px;
    background: transparent;
    color: var(--text-2);
    font-size: 0.79rem;
    font-weight: 600;
    cursor: pointer;
    transition: transform 90ms ease, background-color 140ms ease;
  }

  .zoom button:hover {
    background: var(--accent-soft);
    color: var(--text);
  }

  .zoom button:active {
    transform: scale(0.94);
  }

  .toast {
    position: absolute;
    top: 62px;
    left: 50%;
    z-index: 25;
    transform: translateX(-50%);
    padding: 8px 14px;
    border: 1px solid var(--hairline);
    border-radius: 999px;
    background: var(--material-thick);
    backdrop-filter: blur(var(--material-blur)) saturate(180%);
    box-shadow: var(--shadow-lift);
    color: var(--text-2);
  }

  kbd {
    padding: 1px 5px;
    border: 1px solid var(--hairline);
    border-radius: 5px;
    background: color-mix(in srgb, var(--text) 6%, transparent);
    font: 0.68rem var(--mono);
  }

  /* --- Popover contents -------------------------------------------------- */

  .pop-head {
    margin: 10px 0 5px;
    color: var(--text-3);
  }

  .pop-head:first-child {
    margin-top: 0;
  }

  .pop-line {
    margin: 0 0 4px;
    color: var(--text-2);
  }

  .pop-list {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 3px;
    color: var(--text-2);
  }

  .pop-list b {
    color: var(--text-3);
    font-family: var(--mono);
    font-size: 0.68rem;
    font-weight: 500;
  }

  .pop-choice {
    display: block;
    width: 100%;
    padding: 7px 9px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--text-2);
    font-size: 0.82rem;
    text-align: left;
    cursor: pointer;
  }

  .pop-choice:hover {
    background: var(--accent-soft);
  }

  .pop-choice.on {
    color: var(--text);
    font-weight: 620;
    background: var(--accent-soft);
  }

  .legend {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 5px;
  }

  .legend li {
    display: flex;
    align-items: center;
    gap: 9px;
    color: var(--text-2);
  }

  /* Swatches mirror the real canvas strokes exactly, dashes included. */
  .legend i {
    flex: none;
    width: 22px;
    border-radius: 999px;
  }

  .swatch {
    height: 3px;
  }

  .swatch.code {
    background: var(--calls);
  }

  .swatch.indirect {
    background: repeating-linear-gradient(90deg, var(--indirect) 0 7px, transparent 7px 12px);
  }

  .swatch.bridge {
    background: repeating-linear-gradient(90deg, var(--bridge) 0 2px, transparent 2px 7px);
  }

  .swatch.group {
    background: var(--group);
  }

  .gauge {
    background: var(--text-3);
  }

  .gauge.weak {
    height: 2px;
  }

  .gauge.medium {
    height: 3px;
  }

  .gauge.strong {
    height: 4px;
  }

  /* --- Inspector --------------------------------------------------------- */

  .ins-head {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 16px 18px 0;
  }

  .ins-id {
    min-width: 0;
    flex: 1 1 auto;
  }

  .eyebrow {
    margin: 0 0 6px;
    color: var(--text-3);
  }

  .eyebrow.hot {
    color: var(--warn);
  }

  .ins-head h2 {
    margin: 0 0 3px;
    overflow-wrap: anywhere;
  }

  .ins-head code {
    color: var(--text-3);
    overflow-wrap: anywhere;
  }

  .close {
    flex: none;
    width: 28px;
    height: 28px;
    border: 0;
    border-radius: 8px;
    background: color-mix(in srgb, var(--text) 7%, transparent);
    color: var(--text-2);
    cursor: pointer;
    transition: transform 90ms ease, background-color 140ms ease;
  }

  .close:hover {
    background: color-mix(in srgb, var(--text) 13%, transparent);
    color: var(--text);
  }

  .close:active {
    transform: scale(0.9);
  }

  .lead {
    margin: 12px 18px 14px;
    color: var(--text-2);
  }

  .stats {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    border-top: 1px solid var(--hairline);
    border-bottom: 1px solid var(--hairline);
  }

  .stats div {
    min-width: 0;
    padding: 12px 16px;
    border-right: 1px solid var(--hairline);
  }

  .stats div:last-child {
    border-right: 0;
  }

  .stats strong {
    display: block;
    color: var(--text);
    font-size: 1.34rem;
    font-weight: 640;
    letter-spacing: -0.022em;
    font-variant-numeric: tabular-nums;
  }

  .stats strong.terse {
    font-size: 0.94rem;
    letter-spacing: -0.01em;
    padding-top: 7px;
  }

  .stats span {
    display: block;
    margin-top: 2px;
    color: var(--text-3);
  }

  .ins-body {
    padding: 4px 18px 22px;
  }

  .ins-body h3 {
    margin: 20px 0 9px;
    color: var(--text-3);
    font-weight: 680;
  }

  .dim {
    color: var(--text-3);
  }

  .change {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 5px 12px;
    margin: 0;
    padding: 11px 12px;
    border: 1px solid var(--hairline);
    border-radius: 11px;
    background: color-mix(in srgb, var(--text) 3%, transparent);
  }

  .change dt {
    color: var(--text-3);
    font-weight: 640;
  }

  .change dd {
    margin: 0;
    color: var(--text-2);
  }

  .diffline {
    display: flex;
    align-items: baseline;
    gap: 10px;
    margin: 9px 0 0;
  }

  .diffline b {
    color: var(--add);
  }

  .diffline i {
    color: var(--del);
    font-style: normal;
  }

  .diffline span {
    color: var(--text-3);
  }

  .partners {
    display: grid;
    gap: 10px;
  }

  .partners button {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 5px 12px;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--text);
    text-align: left;
    cursor: pointer;
  }

  .partners button > span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .partners em {
    color: var(--text-3);
    font-style: normal;
    font-variant-numeric: tabular-nums;
  }

  .partners b {
    grid-column: 1 / -1;
    height: 5px;
    overflow: hidden;
    border-radius: 999px;
    background: var(--hairline);
  }

  .partners i {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--accent);
  }

  .partners button:hover i {
    background: color-mix(in srgb, var(--accent) 80%, var(--text));
  }

  .commits {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 11px;
  }

  .commits li {
    display: grid;
    grid-template-columns: 60px minmax(0, 1fr);
    gap: 2px 12px;
  }

  .commits b {
    color: var(--accent);
    font-weight: 620;
  }

  .commits span {
    color: var(--text);
  }

  .commits em {
    grid-column: 2;
    color: var(--text-3);
    font-style: normal;
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .chips button {
    padding: 5px 9px;
    border: 1px solid var(--hairline);
    border-radius: 8px;
    background: transparent;
    color: var(--text-2);
    cursor: pointer;
    transition: transform 90ms ease, border-color 140ms ease, color 140ms ease;
  }

  .chips button:hover {
    color: var(--text);
    border-color: var(--accent);
  }

  .chips button:active {
    transform: scale(0.95);
  }

  .open {
    width: 100%;
    margin-top: 22px;
    padding: 11px 14px;
    border: 0;
    border-radius: 11px;
    background: var(--accent);
    color: var(--accent-ink);
    font-size: 0.86rem;
    font-weight: 640;
    letter-spacing: -0.006em;
    cursor: pointer;
    transition: transform 90ms ease, opacity 140ms ease;
  }

  .open:active {
    transform: scale(0.985);
  }

  .open:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  /* --- Canvas ------------------------------------------------------------ */

  :global(.svelte-flow) {
    background: transparent;
  }

  :global(.svelte-flow__node) {
    /* Positions are sprung in JS; a CSS transition here would fight the drag. */
    transition: none;
  }

  /* An edge says four things: which way it points (arrowhead), how strongly the
     two files are coupled (width), what kind of link it is (colour), and
     whether it is direct or collapsed (solid or dashed). */
  :global(.svelte-flow__edge-path) {
    stroke: var(--edge-color, var(--group));
    stroke-width: var(--edge-width, 1.6);
    stroke-linecap: round;
    stroke-linejoin: round;
    transition: stroke-width 160ms ease, opacity 160ms ease;
  }

  :global(.svelte-flow__edge.strength-weak) {
    --edge-width: 1.6;
  }

  :global(.svelte-flow__edge.strength-medium) {
    --edge-width: 2.6;
  }

  :global(.svelte-flow__edge.strength-strong) {
    --edge-width: 4;
  }

  :global(.svelte-flow__edge.code-edge) {
    --edge-color: var(--calls);
  }

  :global(.svelte-flow__edge.indirect-edge) {
    --edge-color: var(--indirect);
  }

  :global(.svelte-flow__edge.bridge-edge) {
    --edge-color: var(--bridge);
    --edge-width: 2.4;
  }

  :global(.svelte-flow__edge.group-edge) {
    --edge-color: var(--group);
    --edge-width: 1.4;
  }

  :global(.svelte-flow__edge.indirect-edge .svelte-flow__edge-path) {
    stroke-dasharray: 7 9;
  }

  :global(.svelte-flow__edge.bridge-edge .svelte-flow__edge-path) {
    stroke-dasharray: 2 7;
  }

  /* Focus only changes emphasis, never colour: the file's own links come
     forward, the rest recede, and direction stays readable throughout. */
  :global(.svelte-flow__edge.rel-neutral) {
    opacity: 0.78;
  }

  :global(.svelte-flow__edge.rel-unrelated) {
    opacity: 0.12;
  }

  :global(.svelte-flow__edge.rel-outgoing),
  :global(.svelte-flow__edge.rel-incoming) {
    opacity: 1;
  }

  :global(.svelte-flow__edge.rel-outgoing .svelte-flow__edge-path),
  :global(.svelte-flow__edge.rel-incoming .svelte-flow__edge-path) {
    stroke-width: calc(var(--edge-width, 1.6) + 1px);
    filter: drop-shadow(0 0 6px color-mix(in srgb, var(--edge-color) 55%, transparent));
  }

  :global(.svelte-flow__edge-text),
  :global(.svelte-flow__edge-textbg) {
    display: none;
  }

  /* --- Adaptations -------------------------------------------------------- */

  @media (max-width: 900px) {
    .chrome.top {
      flex-wrap: wrap;
      gap: 8px;
    }

    .chrome.top :global(.segmented) {
      order: 3;
      width: 100%;
    }

    .chrome.top :global(.segmented button) {
      flex: 1 1 0;
    }

    .cluster.end .pill:not(.icon) {
      max-width: 42vw;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .chrome.bottom {
      flex-direction: column;
      align-items: stretch;
      gap: 8px;
    }

    .zoom {
      align-self: flex-start;
    }
  }

  @media (max-width: 560px) {
    .cluster.end .pill:not(.icon):not(:last-of-type) {
      display: none;
    }

    .stats {
      grid-template-columns: 1fr 1fr;
    }

    .stats div:nth-child(2) {
      border-right: 0;
    }

    .stats div:last-child {
      grid-column: 1 / -1;
      border-top: 1px solid var(--hairline);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .pill,
    .zoom button,
    .close,
    .chips button,
    .open {
      transition: background-color 140ms ease, color 140ms ease;
      transform: none !important;
    }

    .chrome.bottom {
      transition: opacity 160ms ease;
      transform: none;
    }
  }
</style>

