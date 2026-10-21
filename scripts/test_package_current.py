import io
from pathlib import Path
import struct
import tempfile
import unittest
import zipfile
from package_current import MAGIC, package

class PackageTests(unittest.TestCase):
    def test_fresh_code_and_selected_model(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            mem = io.BytesIO()
            with zipfile.ZipFile(mem, 'w') as z:
                for name in ('model.param', 'model.bin', 'ncnnbridge.dll'):
                    z.writestr(name, b'weights'*100)
            bundle = mem.getvalue()
            (root/'fresh').write_bytes(b'MZnew application code')
            (root/'donor').write_bytes(b'MZstale code'+bundle+MAGIC+struct.pack('<Q', len(bundle)))
            package(root/'fresh', root/'donor', root/'result.exe')
            result = (root/'result.exe').read_bytes()
            self.assertTrue(result.startswith(b'MZnew application code'))
            self.assertNotIn(b'stale code', result)

if __name__ == '__main__':
    unittest.main()
