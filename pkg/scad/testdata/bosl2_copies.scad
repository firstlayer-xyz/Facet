include <BOSL2/std.scad>

// A 3x2 tile of pegs, a mirrored bracket pair, and an X-flipped block pair.
grid_copies(spacing=[12, 12], n=[3, 2]) cyl(h=6, r=2);
fwd(30) mirror_copy([1, 0, 0]) right(8) cuboid([6, 4, 4]);
back(30) xflip_copy() right(5) cuboid([3, 3, 8]);
