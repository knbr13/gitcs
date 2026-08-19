package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
)

type graphViewData struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type graphPageData struct {
	Graph template.JS
}

func writeGraphHTML(graph *Graph) (string, error) {
	nodes := make([]Node, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	payload, err := json.Marshal(graphViewData{Nodes: nodes, Edges: graph.Edges})
	if err != nil {
		return "", fmt.Errorf("encode graph: %w", err)
	}

	page, err := template.New("graph").Parse(graphHTML)
	if err != nil {
		return "", fmt.Errorf("parse graph page: %w", err)
	}

	file, err := os.CreateTemp("", "gitcs-map-*.html")
	if err != nil {
		return "", fmt.Errorf("create graph page: %w", err)
	}
	path := file.Name()

	if err := page.Execute(file, graphPageData{Graph: template.JS(payload)}); err != nil {
		file.Close()
		return "", fmt.Errorf("write graph page: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close graph page: %w", err)
	}

	return path, nil
}

func openGraphInBrowser(path string) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	urlPath := filepath.ToSlash(absolutePath)
	if runtime.GOOS == "windows" {
		urlPath = "/" + urlPath
	}
	pageURL := (&url.URL{Scheme: "file", Path: urlPath}).String()

	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", pageURL)
	case "darwin":
		command = exec.Command("open", pageURL)
	default:
		command = exec.Command("xdg-open", pageURL)
	}

	return command.Start()
}

const graphHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>gitcs project map</title>
  <style>
    :root { color-scheme: dark; font-family: Inter, ui-sans-serif, system-ui, sans-serif; }
    * { box-sizing: border-box; }
    body { margin: 0; color: #e8edf6; background: #0b0d12; }
    header { position: sticky; top: 0; z-index: 5; display: flex; align-items: center; justify-content: space-between; padding: 18px 24px; border-bottom: 1px solid #252b38; background: rgba(11, 13, 18, .94); backdrop-filter: blur(12px); }
    h1 { margin: 0; font-size: 18px; letter-spacing: -.02em; }
    .summary { color: #9ca8bb; font-size: 13px; }
    #viewport { overflow: auto; min-height: calc(100vh - 65px); }
    #board { position: relative; min-width: 100%; min-height: calc(100vh - 65px); }
    #edges { position: absolute; inset: 0; overflow: visible; pointer-events: none; }
    .edge { fill: none; stroke: #65728a; stroke-width: 1.6; opacity: .72; }
    .edge.secondary { stroke-dasharray: 6 6; opacity: .34; }
    .card { position: absolute; width: 250px; min-height: 78px; padding: 14px 16px; color: inherit; text-decoration: none; border: 1px solid #374157; border-radius: 14px; background: linear-gradient(145deg, #1b202a, #141820); box-shadow: 0 12px 28px rgba(0, 0, 0, .28); cursor: grab; touch-action: none; user-select: none; transition: border-color .15s, transform .15s, box-shadow .15s; }
    .card:hover { z-index: 2; transform: translateY(-2px); border-color: #70a7ff; box-shadow: 0 16px 38px rgba(0, 0, 0, .4); }
    .card.dragging { z-index: 3; cursor: grabbing; transform: none; border-color: #70a7ff; box-shadow: 0 18px 44px rgba(0, 0, 0, .48); }
    .card.root { border-color: #3d70ad; background: linear-gradient(145deg, #18273a, #141a24); }
    .card.isolated { border-style: dashed; opacity: .78; }
    .title { overflow: hidden; font-weight: 700; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
    .path { margin-top: 6px; overflow: hidden; color: #95a1b4; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
    .tags { display: flex; gap: 6px; margin-top: 10px; }
    .tag { padding: 3px 7px; border: 1px solid #344057; border-radius: 999px; color: #aebbd0; background: #202633; font-size: 10px; }
    .tag.status { color: #d3b67a; border-color: #594a2c; background: #2a2419; }
    .empty { padding: 80px; color: #96a2b5; text-align: center; }
  </style>
</head>
<body>
  <header>
    <h1>gitcs project map</h1>
    <div id="summary" class="summary"></div>
  </header>
  <main id="viewport">
    <div id="board">
      <svg id="edges" aria-hidden="true">
        <defs><marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 Z" fill="#65728a"></path></marker></defs>
      </svg>
    </div>
  </main>
  <script>
    const graph = {{.Graph}};
    const board = document.getElementById("board");
    const edgeLayer = document.getElementById("edges");
    const summary = document.getElementById("summary");
    summary.textContent = graph.nodes.length + " cards · " + graph.edges.length + " connections";

    if (graph.nodes.length === 0) {
      board.innerHTML = '<div class="empty">No supported source files found.</div>';
    } else {
      const byID = new Map(graph.nodes.map(node => [node.id, node]));
      const allIncoming = new Map(graph.nodes.map(node => [node.id, 0]));
      const allOutgoing = new Map(graph.nodes.map(node => [node.id, []]));
      for (const edge of graph.edges) {
        if (!byID.has(edge.from) || !byID.has(edge.to)) continue;
        allIncoming.set(edge.to, allIncoming.get(edge.to) + 1);
        allOutgoing.get(edge.from).push(edge.to);
      }

      let roots = graph.nodes.filter(node => node.isRoot);
      if (roots.length === 0) {
        roots = graph.nodes.filter(node => allIncoming.get(node.id) === 0 && !node.id.endsWith("_test.go"));
      }
      if (roots.length === 0) roots = [graph.nodes[0]];
      roots.sort((a, b) => a.id.localeCompare(b.id));

      const visibleIDs = new Set(roots.map(node => node.id));
      const parent = new Map();
      const depth = new Map(roots.map(node => [node.id, 0]));
      const children = new Map(graph.nodes.map(node => [node.id, []]));
      const primaryEdges = new Set();
      const queue = roots.map(node => node.id);

      for (let index = 0; index < queue.length; index++) {
        const from = queue[index];
        for (const to of allOutgoing.get(from)) {
          if (visibleIDs.has(to)) continue;
          visibleIDs.add(to);
          parent.set(to, from);
          children.get(from).push(to);
          depth.set(to, depth.get(from) + 1);
          primaryEdges.add(from + "→" + to);
          queue.push(to);
        }
      }

      const visibleNodes = graph.nodes.filter(node => visibleIDs.has(node.id));
      const visibleEdges = graph.edges.filter(edge => visibleIDs.has(edge.from) && visibleIDs.has(edge.to));
      const incoming = new Map(visibleNodes.map(node => [node.id, 0]));
      const outgoing = new Map(visibleNodes.map(node => [node.id, []]));
      const degree = new Map(visibleNodes.map(node => [node.id, 0]));
      for (const edge of visibleEdges) {
        incoming.set(edge.to, incoming.get(edge.to) + 1);
        outgoing.get(edge.from).push(edge.to);
        degree.set(edge.from, degree.get(edge.from) + 1);
        degree.set(edge.to, degree.get(edge.to) + 1);
      }
      summary.textContent = visibleNodes.length + " of " + graph.nodes.length + " cards · " + visibleEdges.length + " connections · rooted view";

      const cardWidth = 250, cardHeight = 92, siblingGap = 54, rowGap = 112, padding = 70;
      const subtreeWidths = new Map();
      function measureSubtree(id) {
        const nodeChildren = children.get(id);
        if (nodeChildren.length === 0) {
          subtreeWidths.set(id, cardWidth);
          return cardWidth;
        }
        const childrenWidth = nodeChildren.reduce((total, child, index) => {
          return total + measureSubtree(child) + (index === 0 ? 0 : siblingGap);
        }, 0);
        const measured = Math.max(cardWidth, childrenWidth);
        subtreeWidths.set(id, measured);
        return measured;
      }

      const rootWidths = roots.map(root => measureSubtree(root.id));
      const forestWidth = rootWidths.reduce((total, rootWidth, index) => total + rootWidth + (index === 0 ? 0 : siblingGap), 0);
      const maxDepth = Math.max(...Array.from(depth.values()));
      const width = Math.max(900, forestWidth + padding * 2);
      const height = Math.max(600, padding * 2 + (maxDepth + 1) * cardHeight + maxDepth * rowGap);
      const positions = new Map();

      function placeSubtree(id, left) {
        const subtreeWidth = subtreeWidths.get(id);
        positions.set(id, {
          x: left + (subtreeWidth - cardWidth) / 2,
          y: padding + depth.get(id) * (cardHeight + rowGap)
        });
        let childLeft = left;
        for (const child of children.get(id)) {
          placeSubtree(child, childLeft);
          childLeft += subtreeWidths.get(child) + siblingGap;
        }
      }

      let rootLeft = (width - forestWidth) / 2;
      roots.forEach((root, index) => {
        placeSubtree(root.id, rootLeft);
        rootLeft += rootWidths[index] + siblingGap;
      });

      board.style.width = width + "px";
      board.style.height = height + "px";
      edgeLayer.setAttribute("width", width);
      edgeLayer.setAttribute("height", height);
      edgeLayer.setAttribute("viewBox", "0 0 " + width + " " + height);

      const renderedEdges = [];
      for (const edge of visibleEdges) {
        const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
        const isPrimary = primaryEdges.has(edge.from + "→" + edge.to);
        path.setAttribute("class", isPrimary ? "edge" : "edge secondary");
        path.setAttribute("marker-end", "url(#arrow)");
        edgeLayer.appendChild(path);
        renderedEdges.push({ edge, path });
      }

      function updateEdgePaths() {
        for (const rendered of renderedEdges) {
          const from = positions.get(rendered.edge.from), to = positions.get(rendered.edge.to);
          if (!from || !to) continue;
          const startX = from.x + cardWidth / 2, startY = from.y + cardHeight;
          const endX = to.x + cardWidth / 2, endY = to.y;
          const bend = Math.max(55, Math.abs(endY - startY) * .45);
          rendered.path.setAttribute("d", "M " + startX + " " + startY + " C " + startX + " " + (startY + bend) + ", " + endX + " " + (endY - bend) + ", " + endX + " " + endY);
        }
      }
      updateEdgePaths();

      for (const node of visibleNodes) {
        const position = positions.get(node.id);
        const card = document.createElement("a");
        card.className = "card";
        if (node.isRoot) card.classList.add("root");
        if (degree.get(node.id) === 0 && !node.isRoot) card.classList.add("isolated");
        card.style.left = position.x + "px";
        card.style.top = position.y + "px";
        card.href = encodeURI("vscode://file/" + node.path.replace(/\\/g, "/"));
        card.title = "Open " + node.path;
        card.draggable = false;

        let drag = null;
        let suppressClick = false;
        card.addEventListener("pointerdown", event => {
          if (event.button !== 0) return;
          drag = {
            pointerX: event.clientX,
            pointerY: event.clientY,
            cardX: position.x,
            cardY: position.y,
            moved: false
          };
          card.setPointerCapture(event.pointerId);
          card.classList.add("dragging");
        });
        card.addEventListener("pointermove", event => {
          if (!drag || !card.hasPointerCapture(event.pointerId)) return;
          const deltaX = event.clientX - drag.pointerX;
          const deltaY = event.clientY - drag.pointerY;
          if (Math.abs(deltaX) > 3 || Math.abs(deltaY) > 3) drag.moved = true;
          position.x = Math.max(20, Math.min(width - cardWidth - 20, drag.cardX + deltaX));
          position.y = Math.max(20, Math.min(height - cardHeight - 20, drag.cardY + deltaY));
          card.style.left = position.x + "px";
          card.style.top = position.y + "px";
          updateEdgePaths();
          event.preventDefault();
        });
        card.addEventListener("pointerup", event => {
          if (!drag) return;
          suppressClick = drag.moved;
          drag = null;
          card.classList.remove("dragging");
          card.releasePointerCapture(event.pointerId);
          if (suppressClick) setTimeout(() => { suppressClick = false; }, 0);
        });
        card.addEventListener("pointercancel", () => {
          drag = null;
          card.classList.remove("dragging");
        });
        card.addEventListener("click", event => {
          if (suppressClick) event.preventDefault();
        });

        const title = document.createElement("div");
        title.className = "title";
        title.textContent = node.label || node.id;
        const path = document.createElement("div");
        path.className = "path";
        path.textContent = node.id;
        const tags = document.createElement("div");
        tags.className = "tags";
        const language = document.createElement("span");
        language.className = "tag";
        language.textContent = node.language || "source";
        tags.appendChild(language);
        if (node.isRoot) {
          const entry = document.createElement("span");
          entry.className = "tag status";
          entry.textContent = "entry point";
          tags.appendChild(entry);
        } else if (degree.get(node.id) === 0) {
          const isolated = document.createElement("span");
          isolated.className = "tag status";
          isolated.textContent = "isolated";
          tags.appendChild(isolated);
        }
        card.append(title, path, tags);
        board.appendChild(card);
      }
    }
  </script>
</body>
</html>`
