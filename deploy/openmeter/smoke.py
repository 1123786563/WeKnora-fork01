"""Exercise real ingest, asynchronous metering, and duplicate delivery."""

import json
import time
import urllib.parse
import urllib.request
import uuid

BASE_URL = "http://127.0.0.1:48888"
subject = "weknora-smoke-" + str(uuid.uuid4())
event = {
    "specversion": "1.0",
    "type": "request",
    "id": subject,
    "source": "weknora-docker-smoke",
    "subject": subject,
    "data": {"method": "GET", "route": "/smoke"},
}
for _ in range(2):
    request = urllib.request.Request(
        BASE_URL + "/api/v1/events",
        data=json.dumps(event).encode(),
        headers={"Content-Type": "application/cloudevents+json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=10) as response:
        assert response.status == 204, response.status

query = BASE_URL + "/api/v1/meters/api_requests_total/query?" + urllib.parse.urlencode({"subject": subject})
consecutive = 0
for _ in range(45):
    with urllib.request.urlopen(query, timeout=10) as response:
        result = json.load(response)
    value = sum(row["value"] for row in result.get("data", []))
    if value > 1:
        raise RuntimeError(f"Duplicate was counted: {result}")
    consecutive = consecutive + 1 if value == 1 else 0
    if consecutive >= 5:
        print(json.dumps({"subject": subject, "deliveries": 2, "metered_value": value, "status": "passed"}))
        break
    time.sleep(1)
else:
    raise RuntimeError(f"Usage did not converge to 1: {result}")
