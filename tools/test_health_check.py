#!/usr/bin/env python3
import os
import platform
import sys
import unittest
from unittest.mock import patch, mock_open

sys.path.insert(0, os.path.dirname(__file__))
from health_check import (
    check_memory_usage,
    check_load_average,
    check_disk_usage,
    _get_memory_info_linux,
)


class TestCheckMemoryUsage(unittest.TestCase):
    def test_linux_memory_usage(self):
        proc_meminfo = "MemTotal:       16384000 kB\nMemAvailable:    8192000 kB\n"
        with patch("health_check.platform.system", return_value="Linux"), \
             patch("builtins.open", mock_open(read_data=proc_meminfo)):
            status, detail, value = check_memory_usage()
            self.assertIn(status, ("OK", "WARNING", "CRITICAL"))
            self.assertIsInstance(value, float)

    def test_load_average_posix(self):
        with patch("health_check.platform.system", return_value="Darwin"), \
             patch.object(os, "getloadavg", return_value=(1.0, 2.0, 3.0)), \
             patch.object(os, "cpu_count", return_value=4):
            status, detail, value = check_load_average()
            self.assertEqual(status, "OK")
            self.assertIn("Load:", detail)

    def test_load_average_linux(self):
        loadavg_data = "1.50 2.00 3.00 1/500 12345\n"
        with patch("health_check.platform.system", return_value="Linux"), \
             patch("os.path.exists", return_value=True), \
             patch("builtins.open", mock_open(read_data=loadavg_data)), \
             patch.object(os, "cpu_count", return_value=4):
            status, detail, value = check_load_average()
            self.assertEqual(status, "OK")
            self.assertIn("Load:", detail)




if __name__ == "__main__":
    unittest.main()
