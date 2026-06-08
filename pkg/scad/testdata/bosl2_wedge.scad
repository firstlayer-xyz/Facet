include <BOSL2/std.scad>

// A corner-anchored ramp and a centered one.
wedge([20, 12, 8]);
right(40) wedge([16, 16, 10], center=true);
