"""Roger enclosure, v0 draft.

Parametric CadQuery model of the DIY-tier shell. Running this file writes
STEP and STL files for each printed part into ./step and ./stl, plus
preview.svg.

    pip install cadquery
    python enclosure.py

The shape follows the concept model (design/model): a rounded slab with a
round speaker grille above a big talk button, a stub antenna and a volume
knob on top, and a strap lug on the side.

THIS HAS NOT BEEN TEST-PRINTED. Every dimension that depends on a bought
part is a named parameter below. Measure your parts and adjust before you
print, and print the front shell first: it carries all the tight fits.

Coordinates: X is width, Y is height (up is +), Z is depth (front is +).
The origin is the centre of the body. Units are millimetres.
"""

import math
import os

import cadquery as cq

# ---- Body ------------------------------------------------------------------
W, H = 68.0, 118.0      # width and height, from the concept model
D = 32.0                # depth. The concept is 30; 32 gives the button room.
WALL = 2.4
CORNER_R = 12.0         # vertical edges
EDGE_R = 5.0            # front and back face edges
LIP_H = 2.0             # height of the alignment lip on the back shell
LIP_T = 1.2
FIT = 0.2               # clearance between mating printed parts

# ---- Speaker: 40 mm slim speaker, 8 ohm, no deeper than 8 mm ----------------
SPK_Y = 20.0
SPK_D = 40.0
SPK_RING_H = 4.0        # locating ring behind the grille
GRILLE_HOLE_D = 3.4
GRILLE_RINGS = [(0.0, 1), (6.2, 6), (12.4, 12), (17.6, 18)]  # (radius, count)

# ---- Talk button: 30 mm snap-in arcade button -------------------------------
BTN_Y = -31.0
BTN_HOLE_D = 30.0       # most 30 mm buttons want 29.5-30.0

# ---- Microphone port and status LED -----------------------------------------
MIC_Y = -9.0
MIC_HOLE_D = 1.6
LED_Y = 50.5
LED_HOLE_D = 3.1        # 3 mm LED, or leave empty

# ---- Top: antenna (decorative), volume pot, USB-C ---------------------------
# The antenna socket and pot hole sit on the split line, half in each shell,
# so the pot's bushing is clamped when the shells close and its nut seats on
# the flat part of the top face.
ANT_X, ANT_PEG_D = -17.0, 6.0
POT_X, POT_HOLE_D = 16.0, 7.2      # 9 mm pot with an M7 bushing
USB_W, USB_H = 12.5, 7.0           # opening for a USB-C plug
USB_Z = -D / 2 + WALL + 1.0 + 1.2 + 1.6  # tape + PCB + half the connector

# ---- Screws: four M3 x 12 self-tapping, from the back -----------------------
BOSS_XY = [(24.0, 49.0), (-24.0, 49.0), (24.0, -49.0), (-24.0, -49.0)]
BOSS_D = 7.0
PILOT_D = 2.6
CLEAR_D = 3.4
HEAD_D, HEAD_H = 6.5, 2.2

# ---- Strap lug ---------------------------------------------------------------
LUG_Y = 38.0


def slab(w, h, d, corner_r, edge_r):
    """A rounded slab centred on the origin."""
    s = cq.Workplane("XY").box(w, h, d).edges("|Z").fillet(corner_r)
    if edge_r > 0:
        s = s.edges("#Z").fillet(edge_r)
    return s


def half(solid, front):
    """Keep the front (z > 0) or back (z < 0) half of a solid."""
    cut = cq.Workplane("XY").box(W * 3, H * 3, D).translate((0, 0, D / 2 if front else -D / 2))
    return solid.intersect(cut)


def top_hole(d, x, z=0.0):
    """A round cutter through the top wall (and the lip behind it)."""
    return cq.Workplane("XZ").circle(d / 2).extrude(-WALL * 3).translate((x, H / 2 - WALL * 2, z))


def ring_points(radius, count, phase=0.0):
    if count == 1:
        return [(0.0, 0.0)]
    return [
        (radius * math.cos(2 * math.pi * i / count + phase), radius * math.sin(2 * math.pi * i / count + phase))
        for i in range(count)
    ]


outer = slab(W, H, D, CORNER_R, EDGE_R)
cavity = slab(W - 2 * WALL, H - 2 * WALL, D - 2 * WALL, CORNER_R - WALL, EDGE_R - WALL)
shell = outer.cut(cavity)
inner_front_z = D / 2 - WALL
inner_back_z = -D / 2 + WALL


def make_front():
    front = half(shell, front=True)

    # Rebate the inside of the rim so the back shell's lip slots in.
    rebate = (
        cq.Workplane("XY")
        .box(W - 2 * WALL + 2 * (LIP_T + FIT), H - 2 * WALL + 2 * (LIP_T + FIT), LIP_H + FIT)
        .edges("|Z")
        .fillet(CORNER_R - WALL + LIP_T + FIT)
        .translate((0, 0, (LIP_H + FIT) / 2))
    )
    front = front.cut(rebate)

    # Speaker locating ring on the inside of the front face.
    ring = (
        cq.Workplane("XY")
        .circle(SPK_D / 2 + 2.0)
        .circle(SPK_D / 2 + FIT)
        .extrude(SPK_RING_H)
        .translate((0, SPK_Y, inner_front_z - SPK_RING_H))
    )
    front = front.union(ring)

    # Screw bosses from the inside of the front face down to the split plane.
    for x, y in BOSS_XY:
        boss = cq.Workplane("XY").circle(BOSS_D / 2).extrude(inner_front_z - LIP_H - FIT).translate((x, y, LIP_H + FIT))
        front = front.union(boss)
        pilot = cq.Workplane("XY").circle(PILOT_D / 2).extrude(inner_front_z - 3.0).translate((x, y, LIP_H + FIT))
        front = front.cut(pilot)

    # Openings through the front face.
    through = lambda d, x, y: cq.Workplane("XY").circle(d / 2).extrude(WALL * 3).translate((x, y, inner_front_z - WALL))
    for i, (radius, count) in enumerate(GRILLE_RINGS):
        for gx, gy in ring_points(radius, count, phase=i * 0.2):
            front = front.cut(through(GRILLE_HOLE_D, gx, SPK_Y + gy))
    front = front.cut(through(BTN_HOLE_D, 0, BTN_Y))
    front = front.cut(through(MIC_HOLE_D, 0, MIC_Y))
    front = front.cut(through(LED_HOLE_D, 0, LED_Y))

    # Front halves of the two holes on the split line.
    front = front.cut(top_hole(ANT_PEG_D + FIT, ANT_X)).cut(top_hole(POT_HOLE_D, POT_X))
    return front


def make_back():
    back = half(shell, front=False)

    # Alignment lip that rises past the split plane into the front shell.
    lip_outer = (
        cq.Workplane("XY")
        .box(W - 2 * WALL + 2 * LIP_T, H - 2 * WALL + 2 * LIP_T, LIP_H + 2.0)
        .edges("|Z")
        .fillet(CORNER_R - WALL + LIP_T)
    )
    lip_inner = (
        cq.Workplane("XY")
        .box(W - 2 * WALL, H - 2 * WALL, LIP_H + 4.0)
        .edges("|Z")
        .fillet(CORNER_R - WALL)
    )
    lip = lip_outer.cut(lip_inner).translate((0, 0, (LIP_H + 2.0) / 2 - 2.0))
    back = back.union(lip)

    # Screw posts from the back face up to the split plane, with a clearance
    # hole and a counterbore for the screw head.
    for x, y in BOSS_XY:
        post = cq.Workplane("XY").circle(BOSS_D / 2).extrude(-inner_back_z).translate((x, y, inner_back_z))
        back = back.union(post)
        hole = cq.Workplane("XY").circle(CLEAR_D / 2).extrude(D).translate((x, y, -D / 2 - 1))
        head = cq.Workplane("XY").circle(HEAD_D / 2).extrude(HEAD_H + 1).translate((x, y, -D / 2 - 1))
        back = back.cut(hole).cut(head)

    # Top face: back halves of the antenna socket and pot hole, and the
    # USB-C opening.
    back = back.cut(top_hole(ANT_PEG_D + FIT, ANT_X)).cut(top_hole(POT_HOLE_D, POT_X))
    usb = (
        cq.Workplane("XZ")
        .rect(USB_W, USB_H)
        .extrude(-WALL * 3)
        .edges("|Y")
        .fillet(2.0)
        .translate((0, H / 2 - WALL * 2, USB_Z))
    )
    back = back.cut(usb)

    # Strap lug on the right-hand side.
    lug = (
        cq.Workplane("XY")
        .box(11.0, 14.0, 5.0)
        .edges("|Z")
        .fillet(3.0)
        .faces(">Z")
        .workplane()
        .center(1.5, 0)
        .hole(6.0)
        .translate((W / 2 + 3.5, LUG_Y, -D / 4))
    )
    return back.union(lug)


def make_antenna():
    """Decorative stub antenna. Glue its peg into the socket on the top."""
    # One revolved profile, so the part is a single clean solid. The ball tip
    # is drawn as short straight segments ending in a small flat: a true
    # sphere meshes with a degenerate pole that some slicers reject.
    peg_r, peg_len = ANT_PEG_D / 2, WALL * 2
    profile = [
        (0, -peg_len), (peg_r, -peg_len), (peg_r, 0),  # peg
        (7.0, 0), (6.5, 5.0),                          # collar
        (6.0, 5.0), (4.2, 39.0),                       # tapered stem
    ]
    tip_r, tip_z = 5.2, 43.0
    for i in range(-5, 11):                            # ball tip, bottom to top
        a = math.radians(i * 8.5)
        profile.append((tip_r * math.cos(a), tip_z + tip_r * math.sin(a)))
    profile.append((0, profile[-1][1]))
    return cq.Workplane("XZ").polyline(profile).close().revolve(360, (0, 0, 0), (0, 1, 0))


def make_knob():
    """Volume knob for a 6 mm knurled pot shaft."""
    knob = cq.Workplane("XY").circle(8.5).workplane(offset=8.0).circle(8.0).loft(combine=True)
    for x, y in ring_points(8.3, 10):
        knob = knob.union(cq.Workplane("XY").box(1.2, 1.2, 7.0).translate((x, y, 4.0)))
    bore = cq.Workplane("XY").circle(3.05).extrude(6.5)
    return knob.cut(bore)


def make_bumper():
    """Optional soft band around the split line. Print in TPU."""
    band = slab(W + 4.0, H + 4.0, 13.0, CORNER_R + 4.0, 0)
    return band.cut(slab(W + FIT, H + FIT, 20.0, CORNER_R, 0))


if __name__ == "__main__":
    here = os.path.dirname(os.path.abspath(__file__))
    for sub in ("step", "stl"):
        os.makedirs(os.path.join(here, sub), exist_ok=True)

    parts = {
        "front_shell": make_front(),
        "back_shell": make_back(),
        "antenna": make_antenna(),
        "volume_knob": make_knob(),
        "bumper_band": make_bumper(),
    }
    for name, part in parts.items():
        cq.exporters.export(part, os.path.join(here, "step", name + ".step"))
        cq.exporters.export(part, os.path.join(here, "stl", name + ".stl"), tolerance=0.05, angularTolerance=0.2)
        bb = part.val().BoundingBox()
        print(f"{name:12s} {bb.xlen:6.1f} x {bb.ylen:6.1f} x {bb.zlen:5.1f} mm   volume {part.val().Volume() / 1000:6.1f} cm3")

    # Assembled preview: shells closed, antenna and knob in place.
    top_y = H / 2
    assembled = (
        parts["front_shell"]
        .union(parts["back_shell"])
        .union(parts["antenna"].rotate((0, 0, 0), (1, 0, 0), -90).translate((ANT_X, top_y, 0)))
        .union(parts["volume_knob"].rotate((0, 0, 0), (1, 0, 0), -90).translate((POT_X, top_y + 1.0, 0)))
    )
    preview = os.path.join(here, "preview.svg")
    cq.exporters.export(
        assembled,
        preview,
        opt={"width": 520, "height": 640, "marginLeft": 30, "marginTop": 30, "showAxes": False,
             "projectionDir": (0.45, 0.35, 1.0), "strokeWidth": 0.35, "strokeColor": (23, 50, 44),
             "showHidden": False},
    )
    # Give the drawing its own background so it reads on light and dark pages.
    with open(preview) as f:
        svg = f.read()
    start = svg.index("<svg")
    end = svg.index(">", start) + 1
    svg = svg[:end] + '<rect width="100%" height="100%" fill="#F3ECDF"/>' + svg[end:]
    with open(preview, "w") as f:
        f.write(svg)
    print("wrote step/, stl/ and preview.svg in", here)
