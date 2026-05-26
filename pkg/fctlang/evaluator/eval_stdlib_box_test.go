package evaluator

import (
	"testing"
)

// All Box.* and Solid bounds-derived accessors are exercised against a
// known 20×20×20 cube placed at the origin: bounding box is
// min=(0,0,0), max=(20,20,20). Each test computes a derived value and
// compares it to a literal so we don't have to introspect the result.

const stdlibTestCube20 = `var c = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});`

// ── Box.* face-coordinate accessors ───────────────────────────────────────────

func TestEvalBoxLeft(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Bounds().Left() == 0 mm`)
}

func TestEvalBoxRight(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Bounds().Right() == 20 mm`)
}

func TestEvalBoxFront(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Bounds().Front() == 0 mm`)
}

func TestEvalBoxBack(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Bounds().Back() == 20 mm`)
}

func TestEvalBoxBottom(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Bounds().Bottom() == 0 mm`)
}

func TestEvalBoxTop(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Bounds().Top() == 20 mm`)
}

// ── Box predicates ────────────────────────────────────────────────────────────

func TestEvalBoxContainsBox(t *testing.T) {
	// Inner 10mm cube at (5,5,5) is fully inside outer 20mm cube at origin.
	setup := stdlibTestCube20 + `
    var inner = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Move(v: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});`
	stdlibIfThenCubeWithSetup(t, setup, `c.Bounds().ContainsBox(other: inner.Bounds())`)
	// And the reverse: outer is NOT contained in inner.
	stdlibIfThenCubeWithSetup(t, setup, `!inner.Bounds().ContainsBox(other: c.Bounds())`)
}

// ── Solid.* face-coordinate shorthands ────────────────────────────────────────
// These thin-wrap Bounds().X() — covering them here keeps the
// Bounds-derived accessor surface fully tested in one place.

func TestEvalSolidLeft(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Left() == 0 mm`)
}

func TestEvalSolidRight(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Right() == 20 mm`)
}

func TestEvalSolidFront(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Front() == 0 mm`)
}

func TestEvalSolidBack(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Back() == 20 mm`)
}

func TestEvalSolidBottom(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Bottom() == 0 mm`)
}

func TestEvalSolidTop(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.Top() == 20 mm`)
}

// ── Solid.* corner-position accessors ─────────────────────────────────────────

func TestEvalSolidLeftFrontBottom(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.LeftFrontBottom() == Vec3{x: 0 mm, y: 0 mm, z: 0 mm}`)
}

func TestEvalSolidRightFrontBottom(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.RightFrontBottom() == Vec3{x: 20 mm, y: 0 mm, z: 0 mm}`)
}

func TestEvalSolidLeftBackBottom(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.LeftBackBottom() == Vec3{x: 0 mm, y: 20 mm, z: 0 mm}`)
}

func TestEvalSolidRightBackBottom(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.RightBackBottom() == Vec3{x: 20 mm, y: 20 mm, z: 0 mm}`)
}

func TestEvalSolidLeftFrontTop(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.LeftFrontTop() == Vec3{x: 0 mm, y: 0 mm, z: 20 mm}`)
}

func TestEvalSolidRightFrontTop(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.RightFrontTop() == Vec3{x: 20 mm, y: 0 mm, z: 20 mm}`)
}

func TestEvalSolidLeftBackTop(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.LeftBackTop() == Vec3{x: 0 mm, y: 20 mm, z: 20 mm}`)
}

func TestEvalSolidRightBackTop(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCube20, `c.RightBackTop() == Vec3{x: 20 mm, y: 20 mm, z: 20 mm}`)
}
