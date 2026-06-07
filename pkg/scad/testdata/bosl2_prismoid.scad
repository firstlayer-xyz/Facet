include <BOSL2/std.scad>

// A tapered pedestal and a wedge-ish ramp block.
prismoid(size1=[30, 30], size2=[14, 14], h=18);
right(50) prismoid(size1=[20, 10], size2=[20, 2], h=10);
