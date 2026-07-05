# data/

## waitlist.json (NOT in git — subscriber PII)

The waitlist lives locally at `data/waitlist.json` (gitignored). Signups arrive as
Web3Forms emails in Gmail (subject: `NOXIOAI waitlist signup`) and are synced into
this file. Phase B imports it into Postgres and retires the file.

Format:

```json
{
  "meta": { "updated": "YYYY-MM-DD", "count": 0, "real_count": 0, "test_count": 0 },
  "entries": [
    {
      "email": "person@example.com",
      "received": "ISO-8601 timestamp",
      "visitor_ip": "from the Web3Forms email",
      "origin": "page URL the form was submitted from",
      "test": false,
      "note": "optional"
    }
  ]
}
```

To sync: ask Claude to "sync waitlist" — it scans Gmail for new signup emails and appends them here.
