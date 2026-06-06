include <BOSL2/std.scad>

// A row of posts and a stack of plates.
xcopies(15, 4) cyl(h=12, r=2);
zcopies(n=3, l=20) cuboid([16, 16, 2]);
