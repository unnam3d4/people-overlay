import unittest
from gate_pixel_benchmark import compare


class CandidateQualityGateTests(unittest.TestCase):
    def row(self,tag,scenario,recall,miss=0,switch=0):
        return {"name":scenario,"report_file":f"replay/{tag}-{scenario}.json",
                "tracking":{"recall":recall,"id_switches":switch},"episodes_missed":miss}

    def test_fails_worse_detection(self):
        scenarios=[self.row("v19","cctv",.50),self.row("v20","cctv",.17,miss=4,switch=8)]
        count,failures=compare(scenarios)
        self.assertEqual(count,1)
        self.assertTrue(any("recall" in s for s in failures))

    def test_allows_slight_noise_but_not_missing_baseline(self):
        rows=[self.row("v19","a",.90),self.row("v20","a",.88)]
        self.assertEqual(compare(rows)[1],[])
        self.assertNotEqual(compare(rows+[self.row("v20","b",.99)])[1],[])

    def test_rejects_missing_episode_and_identity_regressions(self):
        r=[self.row("v19","a",.50),self.row("v20","a",.60,miss=2,switch=4)]
        self.assertEqual(len(compare(r)[1]),2)


if __name__=="__main__":
    unittest.main()
