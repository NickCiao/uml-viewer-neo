// layout.js: boxes and arrows in; positions and orthogonal arrow routes out.
// Importers sit above what they import, so arrows point down.
import ELK from './vendor/elk.bundled.cjs';

const elk = new ELK();

const OPTIONS = {
  'elk.algorithm': 'layered',
  'elk.direction': 'DOWN',
  'elk.edgeRouting': 'ORTHOGONAL',
  'elk.spacing.nodeNode': '32',
  'elk.layered.spacing.nodeNodeBetweenLayers': '56',
  'elk.layered.spacing.edgeNodeBetweenLayers': '20',
};

export async function layout(view) {
  const children = [...view.boxes, ...view.libraries].map((n) => ({ id: n.key, width: n.width, height: n.height }));
  if (children.length === 0) return { width: 0, height: 0, nodes: {}, edges: [] };
  const edges = view.arrows.map((a, i) => ({ id: `e${i}`, sources: [a.from], targets: [a.to] }));
  const out = await elk.layout({ id: 'root', layoutOptions: OPTIONS, children, edges });
  const nodes = {};
  for (const c of out.children) nodes[c.id] = { x: c.x, y: c.y, w: c.width, h: c.height };
  return {
    width: out.width, height: out.height, nodes,
    edges: (out.edges || []).map((e) => ({ from: e.sources[0], to: e.targets[0], points: route(e) })),
  };
}

function route(edge) {
  const s = edge.sections && edge.sections[0];
  if (!s) return [];
  return [s.startPoint, ...(s.bendPoints || []), s.endPoint].map((p) => [p.x, p.y]);
}
