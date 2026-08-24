<script>
  import { Handle, Position } from '@xyflow/svelte';
  let { data, selected = false } = $props();
  const statusNames = { A: 'added', M: 'modified', D: 'deleted', R: 'renamed' };
  let activity = $derived(data.activity?.commits30 ?? 0);
  let railWidth = $derived(Math.min(100, Math.max(8, activity * 8)));
  let isScope = $derived(String(data.id ?? '').startsWith('__'));
</script>

<Handle type="target" position={Position.Top} />
<article class:selected class:scope={isScope} class:changed={Boolean(data.change)} class:affected={data.affected} class:root={data.isRoot} data-status={data.change?.status ?? ''} data-area={data.area ?? ''}>
  <div class="topline">
    <span>{data.language || 'source'}</span>
    <em>{activity}</em>
    {#if data.change}
      <strong title={statusNames[data.change.status]}>{data.change.status}</strong>
    {:else if data.affected}
      <strong class="link">link</strong>
    {/if}
  </div>
  <h3>{data.label}</h3>
  <p>{data.change?.summary?.changed ?? data.description}</p>
  <div class="rail"><span style={`width: ${railWidth}%`}></span></div>
  {#if data.change}<div class="diff">+{data.change.additions} <span>-{data.change.deletions}</span></div>{/if}
</article>
<Handle type="source" position={Position.Bottom} />

<style>
  article { position: relative; width: 230px; min-height: 104px; padding: 13px 15px 14px; overflow: hidden; border: 1px solid #1f3344; border-radius: 8px; color: #dce7ef; background: #101923; box-shadow: 0 16px 36px rgba(0, 0, 0, 0.28); transition: border-color 140ms ease, box-shadow 140ms ease, transform 140ms ease, opacity 140ms ease; }
  article:hover, article.selected { border-color: #7dd3fc; box-shadow: 0 0 0 1px rgba(125, 211, 252, 0.24), 0 18px 42px rgba(0, 0, 0, 0.34); }
  article.selected { transform: translateY(-2px); }
  article.root { border-color: #25566b; }
  article.scope { width: 250px; min-height: 118px; border-color: #2b6a80; background: #0b1a25; }
  article.scope[data-area='backend'] { border-color: #9f6828; background: #1a1510; }
  article.scope h3 { font-size: 1.08rem; }
  article.scope .topline em, article.scope .diff { display: none; }
  article.scope .rail span { width: 100% !important; background: #45bfe3; }
  article.scope[data-area='backend'] .rail span { background: #d6933b; }
  article.changed { border-color: #a7672a; background: #15181a; }
  article.affected { border-style: dashed; opacity: 0.78; }
  article[data-status='A'] { border-color: #2f8f66; }
  article[data-status='D'] { border-color: #a4475d; }
  article[data-status='R'] { border-color: #2f84a4; }
  .topline { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 8px; color: #607485; font: 0.66rem ui-monospace, Consolas, monospace; letter-spacing: 0.08em; text-transform: uppercase; }
  .topline em { color: #3ea6c7; font-style: normal; font-weight: 800; }
  .topline strong { color: #f59e42; font-size: 0.76rem; }
  article[data-status='A'] .topline strong { color: #4ade80; }
  article[data-status='D'] .topline strong { color: #fb7185; }
  article[data-status='R'] .topline strong, .topline .link { color: #38bdf8; }
  h3 { margin: 9px 0 4px; color: #f3f7fb; font-size: 0.95rem; overflow-wrap: anywhere; }
  p { margin: 0; color: #8092a3; font-size: 0.72rem; line-height: 1.35; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .rail { height: 4px; margin-top: 12px; overflow: hidden; border-radius: 999px; background: #1b2b37; }
  .rail span { display: block; height: 100%; border-radius: inherit; background: linear-gradient(90deg, #58c7e8, #f5a742); }
  .diff { margin-top: 7px; color: #4ade80; font: 0.68rem ui-monospace, Consolas, monospace; }
  .diff span { margin-left: 7px; color: #fb7185; }
  :global(.svelte-flow__handle) { width: 7px; height: 7px; border: 1px solid #0d141c; background: #5aa3bd; }
</style>
