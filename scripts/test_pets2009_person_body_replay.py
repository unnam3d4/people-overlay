"""Whole-body PETS2009 coordinate and identity regressions; no network needed."""
import unittest
from pets2009_person_body_replay import xml_objects,box_rescale

class PETSBodyGTTests(unittest.TestCase):
    def test_hand_annotated_person_id_and_center_box(self):
        x=b'''<dataset><frame number="0"><objectlist>
        <object id="7"><box h="60" w="20" xc="90" yc="90"/></object>
        <object id="8"><box h="50" w="16" xc="190" yc="110"/></object>
        </objectlist></frame>
        <frame number="1"><objectlist/></frame></dataset>'''
        people=xml_objects(x)
        self.assertEqual(people[0],[("7",(80.,60.,20.,60.)),("8",(182.,85.,16.,50.))])
        self.assertEqual(people[1],[])

    def test_clamp_body_box_mosaics_with_no_cropped_negative_area(self):
        box=box_rescale((-10.,4.,42.,70.),240/768,135/576,240,135,240,135)
        self.assertAlmostEqual(box["X"],240.)
        self.assertAlmostEqual(box["W"],32*240/768)
        self.assertAlmostEqual(box["Y"],135+4*135/576)
        self.assertIsNone(box_rescale((800,800,20,20),240/768,135/576,0,0,240,135))

    def test_reject_empty_gt(self):
        with self.assertRaisesRegex(ValueError,"no whole-person"):
            xml_objects(b'<dataset><frame number="0"/></dataset>')

if __name__=="__main__":unittest.main()
