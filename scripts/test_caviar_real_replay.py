"""Unit contracts for official CAVIAR CVML conversion. No network/video deps."""
import unittest
from caviar_real_replay import annotations, clamp_box, choose_start

XML = b"""<sequence><frame number="0"><objectlist/></frame>
<frame number="1"><objectlist>
<object id="7"><box xc="192" yc="144" w="60" h="120"/></object>
<object id="8"><box x="12" y="32" w="19" h="39"/></object>
</objectlist></frame><frame number="2"><entitylist>
<entity id="19"><box xc="20" yc="35" w="14" h="26"/></entity>
</entitylist></frame></sequence>"""

class CaviarGTTests(unittest.TestCase):
    def test_extract_center_and_corner_objects(self):
        gt = annotations(XML)
        self.assertEqual(gt[0], [])
        self.assertEqual(len(gt[1]), 2)
        self.assertEqual(gt[1][0], ("7", (162., 84., 60., 120.)))
        self.assertEqual(gt[1][1], ("8", (12., 32., 19., 39.)))
        self.assertEqual(gt[2][0], ("19", (13., 22., 14., 26.)))

    def test_scaled_ground_truth_is_clipped_to_source(self):
        rect = clamp_box((-10, 90, 30, 60), 384, 288, 240, 135, 240, 135)
        self.assertAlmostEqual(rect["X"], 240.)
        self.assertAlmostEqual(rect["W"], 20 * 240 / 384)
        self.assertAlmostEqual(rect["Y"], 135 + 90 * 135 / 288)

    def test_event_window_never_exceeds_video(self):
        self.assertEqual(choose_start({200: [1, 2, 3]}, 250, 25, 10), 0)
        self.assertEqual(choose_start({200: [1, 2, 3]}, 1000, 25, 10), 75)

    def test_empty_annotations_rejected(self):
        with self.assertRaisesRegex(ValueError, "no annotated"):
            annotations(b"<sequence><frame number='0'><objectlist/></frame></sequence>")

if __name__ == "__main__":
    unittest.main()
