include <BOSL2/std.scad>

// A plate drilled through, with a wide shallow pocket milled into the top face.
diff()
cuboid([40, 24, 10]) {
    tag("remove") cyl(h=30, d=8);
    position(TOP) tag("remove") cyl(h=6, d=16);
}
