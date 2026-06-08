include <BOSL2/std.scad>

// A five-point star and a chunkier eight-point star, extruded.
linear_extrude(2) star(n=5, r=12, ir=5);
right(40) linear_extrude(2) star(n=8, r=10, ir=7);
