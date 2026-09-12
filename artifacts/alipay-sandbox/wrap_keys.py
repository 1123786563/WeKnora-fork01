#!/usr/bin/env python3
"""Wrap Alipay sandbox bare-base64 keys into PEM files.

Usage: python3 artifacts/alipay-sandbox/wrap_keys.py <bare-base64-in> <pem-out> <public|private>
The sandbox console shows keys as bare base64; the Go provider expects PEM.
"""
import base64, sys, textwrap
src, dst, kind = sys.argv[1], sys.argv[2], sys.argv[3]
body = open(src).read().strip().replace('\n','').replace(' ','')
header = {'public':'PUBLIC KEY','private':'PRIVATE KEY'}[kind]
lines = textwrap.wrap(base64.b64decode(body + '='*(-len(body)%4)).hex() and body, 64) if False else textwrap.wrap(body, 64)
open(dst,'w').write('-----BEGIN %s-----\n%s\n-----END %s-----\n' % (header, '\n'.join(lines), header))
print('wrote', dst)
