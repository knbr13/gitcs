<script>
  import { Handle, Position } from '@xyflow/svelte';
  let { data, selected = false } = $props();
  const statusNames = { A: 'added', M: 'modified', D: 'deleted', R: 'renamed' };
</script>

<Handle type="target" position={Position.Top} />
<article class:selected class:changed={Boolean(data.change)} class:affected={data.affected} class:root={data.isRoot} data-status={data.change?.status ?? ''}>
  <div class="topline">
    <span>{data.language}</span>
    {#if data.change}
      <strong title={statusNames[data.change.status]}>{data.change.status}</strong>
    {:else if data.affected}
      <strong class="link">↔</strong>
    {/if}
  </div>
  <h3>{data.label}</h3>
  <p>{data.description}</p>
  {#if data.change}<div class="diff">+{data.change.additions} <span>−{data.change.deletions}</span></div>{/if}
</article>
<Handle type="source" position={Position.Bottom} />

<style>
  article { position: relative; width: 210px; min-height: 92px; padding: 11px 13px 15px; overflow: hidden; border: 1px solid #445080; border-radius: 8px; color: #f0f3ff; background: linear-gradient(145deg, #303960, #252d4d); box-shadow: 0 8px 24px rgba(0, 0, 0, 0.28); transition: border-color 140ms ease, box-shadow 140ms ease, transform 140ms ease; }
  article::after { position: absolute; right: 12px; bottom: 7px; left: 12px; height: 3px; border-radius: 999px; background: linear-gradient(90deg, #38d9a9 0 24%, #e7eaf7 24% 100%); content: ''; }
  article:hover, article.selected { border-color: #67e8f9; box-shadow: 0 0 0 1px rgba(103, 232, 249, 0.25), 0 12px 32px rgba(0, 0, 0, 0.28); }
  article.selected { transform: translateY(-2px); }
  article.root { border-color: #7586d9; box-shadow: 0 0 0 1px rgba(117, 134, 217, 0.25), 0 8px 24px rgba(0, 0, 0, 0.28); }
  article.changed { border-color: #d8952e; }
  article.affected { border-style: dashed; }
  article[data-status='A'] { border-color: #36a46c; }
  article[data-status='D'] { border-color: #d45b75; }
  article[data-status='R'] { border-color: #3aaed8; }
  .topline { display: flex; align-items: center; justify-content: space-between; color: #6e8fa4; font: 0.64rem ui-monospace, Consolas, monospace; letter-spacing: 0.08em; text-transform: uppercase; }
  .topline strong { color: #fbbf24; font-size: 0.78rem; }
  article[data-status='A'] .topline strong { color: #4ade80; }
  article[data-status='D'] .topline strong { color: #fb7185; }
  article[data-status='R'] .topline strong, .topline .link { color: #38bdf8; }
  h3 { margin: 7px 0 3px; font-size: 0.9rem; overflow-wrap: anywhere; }
  p { margin: 0; color: #c0c8e2; font-size: 0.68rem; line-height: 1.35; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .diff { margin-top: 6px; color: #4ade80; font: 0.64rem ui-monospace, Consolas, monospace; }
  .diff span { margin-left: 7px; color: #fb7185; }
  :global(.svelte-flow__handle) { width: 7px; height: 7px; border: 1px solid #171c31; background: #7b88c9; }
</style>
