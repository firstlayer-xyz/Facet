include <BOSL2/std.scad>

// Extruded 2D profiles: a plate, a hex boss, and a pentagon.
linear_extrude(2) rect([24, 14]);
right(30) linear_extrude(3) hexagon(r=10);
back(30) linear_extrude(2) regular_ngon(n=5, r=8);
