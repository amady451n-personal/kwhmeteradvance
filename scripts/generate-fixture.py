"""Generate a read-only fixture from the basic app's SQLite snapshot.

Usage: python3 scripts/generate-fixture.py /path/to/meters.db > fixture.json
The fixture keeps raw readings only. All derived values are computed in the
frontend so the same domain code can later read the advanced API.
"""
import json
import sqlite3
import sys
from datetime import datetime, timedelta, timezone

WIB = timezone(timedelta(hours=7))


def parse_timestamp(value: str) -> datetime:
    for layout in ("%Y-%m-%d %H:%M:%S.%f", "%Y-%m-%d %H:%M:%S"):
        try:
            return datetime.strptime(value, layout)
        except ValueError:
            continue
    raise ValueError(f"unsupported timestamp: {value}")


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: generate-fixture.py <meters.db>")

    con = sqlite3.connect(sys.argv[1])
    rows = []
    query = """
        select id, meter_name, reading, timestamp, notes, recharge_amount, cost
        from readings
        order by timestamp, id
    """
    for row_id, meter, reading, timestamp, notes, recharge, cost in con.execute(query):
        rows.append(
            {
                "id": row_id,
                "meter_name": meter,
                "reading": reading,
                "timestamp": parse_timestamp(timestamp).replace(tzinfo=WIB).isoformat(),
                "notes": notes,
                "recharge_amount": recharge,
                "cost": cost,
            }
        )

    payload = {
        "source": "snapshot meters.db basic app",
        "generated_at": datetime.now(WIB).replace(microsecond=0).isoformat(),
        "meter": "main",
        "readings": rows,
        "tokens": [],
    }
    json.dump(payload, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
