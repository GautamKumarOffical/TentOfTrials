#!/usr/bin/env python3
"""Tests for benchmark rate-limit bypass functionality."""

import sys
import os
import unittest
from unittest.mock import patch, MagicMock

sys.path.insert(0, os.path.dirname(__file__))
from benchmark import make_request, BYPASS_RATE_LIMIT_HEADER


def get_header_lower(req, header_name):
    """Get a header value from a urllib Request using case-insensitive lookup."""
    for k, v in req.headers.items():
        if k.lower() == header_name.lower():
            return v
    return None


class TestBypassRateLimit(unittest.TestCase):
    """Tests for the X-Benchmark-Bypass-Rate-Limit header."""

    @patch('benchmark.urllib.request.urlopen')
    def test_default_no_bypass_header(self, mock_urlopen):
        """Without bypass flag, no bypass header is sent."""
        mock_response = MagicMock()
        mock_response.status = 200
        mock_response.read.return_value = b'{}'
        mock_response.__enter__ = lambda s: s
        mock_response.__exit__ = MagicMock(return_value=False)
        mock_urlopen.return_value = mock_response

        make_request('http://localhost:8080/test')

        call_args = mock_urlopen.call_args
        req = call_args[0][0]
        self.assertIsNone(get_header_lower(req, BYPASS_RATE_LIMIT_HEADER))

    @patch('benchmark.urllib.request.urlopen')
    def test_bypass_header_present_when_enabled(self, mock_urlopen):
        """With bypass flag, the bypass header is set to 'true'."""
        mock_response = MagicMock()
        mock_response.status = 200
        mock_response.read.return_value = b'{}'
        mock_response.__enter__ = lambda s: s
        mock_response.__exit__ = MagicMock(return_value=False)
        mock_urlopen.return_value = mock_response

        make_request('http://localhost:8080/test', bypass_rate_limit=True)

        call_args = mock_urlopen.call_args
        req = call_args[0][0]
        self.assertEqual(get_header_lower(req, BYPASS_RATE_LIMIT_HEADER), 'true')

    @patch('benchmark.urllib.request.urlopen')
    def test_bypass_does_not_expose_secrets(self, mock_urlopen):
        """Bypass header does not contain auth tokens or secrets."""
        mock_response = MagicMock()
        mock_response.status = 200
        mock_response.read.return_value = b'{}'
        mock_response.__enter__ = lambda s: s
        mock_response.__exit__ = MagicMock(return_value=False)
        mock_urlopen.return_value = mock_response

        make_request('http://localhost:8080/test', bypass_rate_limit=True)

        call_args = mock_urlopen.call_args
        req = call_args[0][0]
        self.assertIsNone(get_header_lower(req, 'Authorization'))
        self.assertIsNone(get_header_lower(req, 'X-API-Key'))

    @patch('benchmark.urllib.request.urlopen')
    def test_existing_headers_preserved_with_bypass(self, mock_urlopen):
        """Existing headers are preserved when bypass is enabled."""
        mock_response = MagicMock()
        mock_response.status = 200
        mock_response.read.return_value = b'{}'
        mock_response.__enter__ = lambda s: s
        mock_response.__exit__ = MagicMock(return_value=False)
        mock_urlopen.return_value = mock_response

        existing_headers = {'Authorization': 'Bearer token123', 'Accept': 'application/json'}
        make_request('http://localhost:8080/test', headers=existing_headers,
                     bypass_rate_limit=True)

        call_args = mock_urlopen.call_args
        req = call_args[0][0]
        self.assertEqual(get_header_lower(req, 'Authorization'), 'Bearer token123')
        self.assertEqual(get_header_lower(req, 'Accept'), 'application/json')
        self.assertEqual(get_header_lower(req, BYPASS_RATE_LIMIT_HEADER), 'true')

    @patch('benchmark.urllib.request.urlopen')
    def test_bypass_flag_default_false(self, mock_urlopen):
        """Default bypass_rate_limit is False."""
        mock_response = MagicMock()
        mock_response.status = 200
        mock_response.read.return_value = b'{}'
        mock_response.__enter__ = lambda s: s
        mock_response.__exit__ = MagicMock(return_value=False)
        mock_urlopen.return_value = mock_response

        make_request('http://localhost:8080/test')

        call_args = mock_urlopen.call_args
        req = call_args[0][0]
        self.assertIsNone(get_header_lower(req, BYPASS_RATE_LIMIT_HEADER))

    @patch('benchmark.urllib.request.urlopen')
    def test_bypass_header_value_is_string_true(self, mock_urlopen):
        """Bypass header value is the string 'true', not a boolean."""
        mock_response = MagicMock()
        mock_response.status = 200
        mock_response.read.return_value = b'{}'
        mock_response.__enter__ = lambda s: s
        mock_response.__exit__ = MagicMock(return_value=False)
        mock_urlopen.return_value = mock_response

        make_request('http://localhost:8080/test', bypass_rate_limit=True)

        call_args = mock_urlopen.call_args
        req = call_args[0][0]
        value = get_header_lower(req, BYPASS_RATE_LIMIT_HEADER)
        self.assertIsInstance(value, str)
        self.assertEqual(value, 'true')


if __name__ == '__main__':
    unittest.main()
