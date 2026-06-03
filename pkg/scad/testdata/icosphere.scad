// Icosphere — FIXED for Facet transpilation (original Wikibooks source was buggy).
// Changes from the original:
//  - optional icoPnts/icoTris default to [] instead of relying on undef truthiness
//  - removed the broken search()-based vertex dedup (the geometry kernel merges
//    coincident vertices; we always add the 3 edge midpoints)
//  - vector math written with explicit components ((p1[0]+p2[0])/2, norm inline),
//    since Facet '+' on lists concatenates rather than adding element-wise
//  - renamed the in-branch verts/tris reassignment to curVerts/curTris (no shadowing)
//  - added a top-level call so it renders standalone
// recursion: 0 = 20 tris, 1 = 80, 2 = 320.
module icosphere(radius = 10, recursion = 2, icoPnts = [], icoTris = []) {
  t = sqrt((5 + sqrt(5)) / 10);
  s = sqrt((5 - sqrt(5)) / 10);

  init = len(icoPnts) == 0 && len(icoTris) == 0;

  verts = [
    [-s, t, 0], [s, t, 0], [-s, -t, 0], [s, -t, 0],
    [0, -s, t], [0, s, t], [0, -s, -t], [0, s, -t],
    [t, 0, -s], [t, 0, s], [-t, 0, -s], [-t, 0, s]];

  tris = [
    [0, 5, 11], [0, 1, 5], [0, 7, 1], [0, 10, 7], [0, 11, 10],
    [1, 9, 5], [5, 4, 11], [11, 2, 10], [10, 6, 7], [7, 8, 1],
    [3, 4, 9], [3, 2, 4], [3, 6, 2], [3, 8, 6], [3, 9, 8],
    [4, 5, 9], [2, 11, 4], [6, 10, 2], [8, 7, 6], [9, 1, 8]];

  if (recursion > 0) {
    curVerts = init ? verts : icoPnts;
    curTris = init ? tris : icoTris;
    newSegments = recurseTris(curVerts, curTris);
    newVerts = newSegments[0];
    newTris = newSegments[1];
    icosphere(radius, recursion - 1, newVerts, newTris);
  } else if (init) {
    scale(radius) polyhedron(verts, tris);
  } else {
    scale(radius) polyhedron(icoPnts, icoTris);
  }
}

// Subdivide one triangle into four, adding the three edge midpoints.
function addTris(verts, tri) = let(
    a = getMiddlePoint(verts[tri[0]], verts[tri[1]]),
    b = getMiddlePoint(verts[tri[1]], verts[tri[2]]),
    c = getMiddlePoint(verts[tri[2]], verts[tri[0]]),
    l = len(verts)
  ) [concat(verts, [a, b, c]),
     [[tri[0], l, l + 2],
      [tri[1], l + 1, l],
      [tri[2], l + 2, l + 1],
      [l, l + 1, l + 2]]];

// One subdivision pass over every triangle (auto step count from len(tris)).
function recurseTris(verts, tris, newTris = [], steps = 0, step = 0) = let(
    stepsCnt = steps > 0 ? steps : len(tris) - 1,
    newSegment = addTris(verts, tris[step]),
    newVerts = newSegment[0],
    newerTris = concat(newTris, newSegment[1])
  ) stepsCnt == step ? [newVerts, newerTris]
                     : recurseTris(newVerts, tris, newerTris, stepsCnt, step + 1);

// Midpoint of two unit-sphere points, projected back to the unit sphere.
function getMiddlePoint(p1, p2) = fixPosition(
    [(p1[0] + p2[0]) / 2, (p1[1] + p2[1]) / 2, (p1[2] + p2[2]) / 2]);

function fixPosition(p) = let(l = sqrt(p[0] * p[0] + p[1] * p[1] + p[2] * p[2]))
    [p[0] / l, p[1] / l, p[2] / l];

icosphere(radius = 10, recursion = 1);
