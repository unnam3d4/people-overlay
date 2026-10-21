"""Offline tests: CVAT landmark decoding and honest real-video metric names."""
import unittest
from github_real_keypoint_replay import face_points_from_cvat
from evaluate_landmark_replay import score, compare

XML = b"""<annotations>
<track id="4" label="face_Keypoints"><points frame="0" points="70,20" outside="0"/><points frame="1" points="72,20" outside="0"/><points frame="2" points="74,20" outside="1"/></track>
<track id="5" label="body"><points frame="0" points="45,80" outside="0"/></track>
</annotations>"""


class TestRealPointReplay(unittest.TestCase):
    def test_only_visible_annotated_face_points(self):
        v = face_points_from_cvat(XML, require_real_length=False)
        self.assertEqual(v[0], [{"id":"track4","x":70.,"y":20.}])
        self.assertEqual(v[1][0]["x"],72.)
        self.assertNotIn(2, v)

    def test_coverage_and_switch_are_not_box_precision(self):
        frame = lambda ms,x,tid: {"timestamp_ms":ms, "landmarks":[{"id":"track4","x":x,"y":20}],
            "tracks":[{"ID":tid,"Box":{"X":x-15,"Y":10,"W":30,"H":100}}], "times":{}}
        sample = {"manifest":{"name":"manual-face","fps":10},"frames":[frame(0,70,1),frame(100,72,2),{"timestamp_ms":200,"landmarks":[],"tracks":[]}]}
        result = score(sample)
        self.assertEqual(result["manually_labelled_face_points"],2)
        self.assertEqual(result["face_landmark_coverage"],1.)
        self.assertEqual(result["id_switches_on_matched_face_landmarks"],1)
        self.assertNotIn("precision",result)

    def test_regression_gate_checks_landmarks_not_bboxes(self):
        a={"name":"g","report_file":"r/v19-g.json","face_landmark_coverage":.9,"id_switches_on_matched_face_landmarks":0}
        b={"name":"g","report_file":"r/v20-g.json","face_landmark_coverage":.6,"id_switches_on_matched_face_landmarks":4}
        self.assertEqual(len(compare([a,b])),2)


if __name__=="__main__":unittest.main()
