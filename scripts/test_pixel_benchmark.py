import unittest
from evaluate_pixel_benchmark import evaluate, match_boxes

B = {"X": 20, "Y": 30, "W": 20, "H": 50}

class BenchmarkEvaluatorTests(unittest.TestCase):
    def test_delay_switch_and_ghost(self):
        frames = []
        for i in range(5):
            frames.append({"timestamp_ms": i*100, "people": [{"id": "p", "box": B}] if i<3 else [],
                           "tracks": [] if i in (0,4) else [{"ID": 1 if i==1 else 2, "Box": B}], "times": {}})
        result = evaluate({"manifest":{"name":"test", "fps": 10}, "frames":frames, "inferences":[]})
        self.assertAlmostEqual(result["tracking"]["precision"], 2/3)
        self.assertAlmostEqual(result["tracking"]["recall"], 2/3)
        self.assertEqual(result["tracking"]["id_switches"], 1)
        self.assertEqual(result["time_to_first_track_ms"]["p50"], 100)
        self.assertEqual(result["ghost_after_disappearance_ms"]["p50"], 100)

    def test_missed_episode_not_dropped(self):
        result = evaluate({"manifest":{"fps":15}, "inferences":[], "frames":[
            {"timestamp_ms":0,"people":[{"id":"p","box":B}],"tracks":[],"times":{}}]})
        self.assertEqual(result["episodes_missed"], 1)
        self.assertIsNone(result["time_to_first_track_ms"]["p50"])

    def test_assignment_is_one_to_one(self):
        pairs = match_boxes([B, B], [B], .5)
        self.assertEqual(len(pairs), 1)

    def test_unremoved_ghost_is_censored(self):
        result = evaluate({"manifest":{"fps":10}, "inferences":[], "frames":[
            {"timestamp_ms":0,"people":[{"id":"p","box":B}],"tracks":[{"ID":1,"Box":B}],"times":{}},
            {"timestamp_ms":100,"people":[],"tracks":[{"ID":1,"Box":B}],"times":{}}]})
        self.assertEqual(result["ghost_censored_events"],1)
        self.assertEqual(result["ghost_after_disappearance_ms"]["count"],0)

    def test_assignment_avoids_greedy_cardinality_loss(self):
        a={"X":0,"Y":0,"W":10,"H":10}
        b={"X":5,"Y":0,"W":10,"H":10}
        p={"X":1,"Y":0,"W":10,"H":10}
        q={"X":-3,"Y":0,"W":10,"H":10}
        self.assertEqual(set(match_boxes([a,b],[p,q],.4)),{(0,1),(1,0)})

if __name__ == "__main__":
    unittest.main()
