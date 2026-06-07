include <BOSL2/std.scad>

// A ring and a fatter ring offset along X.
torus(r_maj=20, r_min=3);
right(60) torus(or=23, ir=15);
