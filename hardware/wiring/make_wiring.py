"""Draws wiring.svg for the DIY tier.

The pin map here mirrors firmware/include/boards/xiao_diy.h. If you change
one, change the other, then run:  python make_wiring.py
"""

INK, CREAM, ORANGE, YELLOW, MINT, MUTE = "#17322C", "#F3ECDF", "#F2622E", "#FFC83D", "#9FF0C4", "#6B827B"
TAGS = {"GND": INK, "3V3": "#C7461A", "BAT+": "#9A6B00", "BAT-": INK}
W, H = 1060, 720
out = []
a = out.append


def box(x, y, w, h, title, fill="#FFFFFF", sub=None):
    a(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="10" fill="{fill}" stroke="{INK}" stroke-width="1.5"/>')
    a(f'<text x="{x + 12}" y="{y + 21}" font-weight="700" fill="{INK}">{title}</text>')
    if sub:
        a(f'<text x="{x + 12}" y="{y + 37}" font-size="11" fill="{MUTE}">{sub}</text>')


def tag(x, y, name, side="right"):
    """A named power pill. side says which way it extends from x."""
    w = 46
    rx = x if side == "right" else x - w
    a(f'<rect x="{rx}" y="{y - 10}" width="{w}" height="20" rx="10" fill="{TAGS[name]}"/>')
    a(f'<text x="{rx + w / 2}" y="{y + 4}" text-anchor="middle" font-size="11" font-weight="700" fill="#FFFFFF">{name}</text>')


def stub(x1, x2, y):
    a(f'<path d="M{x1},{y} L{x2},{y}" stroke="{INK}" stroke-width="1.5"/>')


def wire(x1, x2, y):
    a(f'<path d="M{x1},{y} L{x2},{y}" stroke="{ORANGE}" stroke-width="2.5"/>')
    a(f'<circle cx="{x1}" cy="{y}" r="4" fill="{ORANGE}"/><circle cx="{x2}" cy="{y}" r="4" fill="{ORANGE}"/>')


def text(x, y, t, anchor="start", bold=False, col=INK, size=12):
    fw = ' font-weight="700"' if bold else ""
    a(f'<text x="{x}" y="{y + 4}" text-anchor="{anchor}" font-size="{size}" fill="{col}"{fw}>{t}</text>')


a(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" '
  'font-family="-apple-system,BlinkMacSystemFont,Segoe UI,Helvetica,Arial,sans-serif" font-size="13">')
a("<title>Roger DIY wiring</title>")
a("<desc>Wiring between a XIAO ESP32S3, an INMP441 microphone, a MAX98357A amplifier, a speaker, "
  "a talk button, a volume pot and a battery.</desc>")
a(f'<rect width="{W}" height="{H}" fill="{CREAM}"/>')
a(f'<text x="32" y="42" font-size="20" font-weight="700" fill="{INK}">Roger DIY wiring</text>')
a(f'<text x="32" y="62" fill="{MUTE}">Standard wiring. The pin map lives in firmware/include/boards/xiao_diy.h</text>')

# --- XIAO, centre -------------------------------------------------------------
XL, XR, TOP, ROW = 440, 620, 150, 58
row = lambda i: TOP + i * ROW
box(XL, TOP - 62, XR - XL, ROW * 6 + 96, "XIAO ESP32S3", fill=MINT, sub="USB-C at this end")
GPIO = {"D0": 1, "D1": 2, "D2": 3, "D3": 4, "D4": 5, "D5": 6, "D6": 43, "D7": 44, "D8": 7, "D9": 8, "D10": 9}
for i, p in enumerate(["D0", "D1", "D2", "D3", "D4", "D5", "D6"]):
    text(XL + 12, row(i), p, bold=True, size=14)
    text(XL + 44, row(i), f"GPIO{GPIO[p]}", col=MUTE, size=10)
for i, p in enumerate(["5V", "GND", "3V3", "D10", "D9", "D8", "D7"]):
    text(XR - 12, row(i), p, anchor="end", bold=True, size=14)
    if p in GPIO:
        text(XR - 46, row(i), f"GPIO{GPIO[p]}", anchor="end", col=MUTE, size=10)

# --- Left: talk button, amplifier, speaker, volume pot --------------------------
MX, MW = 180, 180
MR = MX + MW

box(MX, row(0) - 22, MW, 44, "Talk button", fill=YELLOW)
wire(MR, XL, row(0))
stub(MX - 12, MX, row(0))
tag(MX - 12, row(0), "GND", side="left")
text(MX - 66, row(0) - 18, "other leg", anchor="end", col=MUTE, size=10)

box(MX, row(1) - 30, MW, ROW * 3 + 56, "MAX98357A amp")
for i, pin in enumerate(["BCLK", "LRC", "DIN", "SD"]):
    text(MR - 10, row(1 + i), pin, anchor="end")
    wire(MR, XL, row(1 + i))
text(MX + 10, row(2), "Vin")
stub(MX - 12, MX, row(2))
tag(MX - 12, row(2), "BAT+", side="left")
text(MX + 10, row(3), "GND")
stub(MX - 12, MX, row(3))
tag(MX - 12, row(3), "GND", side="left")
text(MX + 10, row(4), "+ and −")
box(20, row(4) - 22, 104, 44, "Speaker", sub="8 Ω, 40 mm")
a(f'<path d="M124,{row(4) - 4} L{MX},{row(4) - 4} M124,{row(4) + 4} L{MX},{row(4) + 4}" stroke="{INK}" stroke-width="1.5"/>')

box(MX, row(5) - 22, MW, 64, "Volume pot")
text(MR - 10, row(5), "wiper", anchor="end")
wire(MR, XL, row(5))
text(MX + 10, row(5) + 12, "end")
stub(MX - 12, MX, row(5) + 12)
tag(MX - 12, row(5) + 12, "3V3", side="left")
text(MX + 10, row(5) + 34, "end")
stub(MX - 12, MX, row(5) + 34)
tag(MX - 12, row(5) + 34, "GND", side="left")

text(XL - 16, row(6), "optional status LED, through 220 Ω to GND", anchor="end", col=MUTE, size=11)
a(f'<path d="M{XL - 10},{row(6)} L{XL},{row(6)}" stroke="{MUTE}" stroke-width="2" stroke-dasharray="3 3"/>')

# --- Right: power pins and microphone -------------------------------------------
RX, RW = 800, 150
text(XR + 16, row(0), "not used", col=MUTE, size=11)
stub(XR, XR + 12, row(1))
tag(XR + 12, row(1), "GND")
stub(XR, XR + 12, row(2))
tag(XR + 12, row(2), "3V3")
text(XR + 16, row(6), "not used", col=MUTE, size=11)

box(RX, row(3) - 46, RW, ROW * 3 + 102, "INMP441 mic")
for i, pin in enumerate(["SD", "WS", "SCK"]):
    text(RX + 10, row(3 + i), pin)
    wire(XR, RX, row(3 + i))
text(RX + 10, row(6) - 10, "VDD")
stub(RX + RW, RX + RW + 12, row(6) - 10)
tag(RX + RW + 12, row(6) - 10, "3V3")
text(RX + 10, row(6) + 16, "GND and L/R")
stub(RX + RW, RX + RW + 12, row(6) + 16)
tag(RX + RW + 12, row(6) + 16, "GND")

# --- Battery ----------------------------------------------------------------------
BY = row(6) + 62
box(XL - 150, BY, XR - XL + 300, 96, "Battery", sub="3.7 V LiPo with a protection circuit")
text(XL - 138, BY + 58, "+   →   slide switch   →")
tag(XL + 2, BY + 58, "BAT+")
text(XL + 56, BY + 58, "= BAT+ pad under the XIAO, and amp Vin", col=MUTE, size=11)
text(XL - 138, BY + 80, "−   →   BAT− pad under the XIAO")

# --- Legend -----------------------------------------------------------------------
a(f'<path d="M32,{H - 28} L64,{H - 28}" stroke="{ORANGE}" stroke-width="2.5"/>')
text(72, H - 28, "signal wire", col=MUTE, size=11)
tag(160, H - 28, "GND")
text(214, H - 28, "pills with the same name are all joined together", col=MUTE, size=11)
a("</svg>")

with open("wiring.svg", "w") as f:
    f.write("\n".join(out) + "\n")
print("wrote wiring.svg")
