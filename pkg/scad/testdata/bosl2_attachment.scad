include <BOSL2/std.scad>

// A post sitting on top of a base, and a peg straddling the right edge.
cuboid([30, 30, 12])
    attach(TOP) cyl(h=16, r=4);

right(40) cuboid([20, 20, 20])
    position(RIGHT+TOP) cuboid([4, 4, 4]);
