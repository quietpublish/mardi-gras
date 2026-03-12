#!/bin/bash
# Fake gt binary for local TUI testing.
# Usage: PATH="$(pwd)/testdata:$PATH" ./mg --path testdata/sample.jsonl
# Responds to the gt subcommands that mg invokes.

case "$1" in
  status)
    cat <<'EOF'
{
  "name": "test-hq",
  "location": "/tmp/gt",
  "agents": [
    {"name":"mayor","address":"mayor/","session":"hq-mayor",
     "role":"coordinator","has_work":false,"unread_mail":1,"state":"idle",
     "agent_alias":"Mayor Adams","first_subject":"Weekly patrol report"}
  ],
  "rigs": [{
    "name":"mardi_gras",
    "polecat_count":2,
    "crew_count":1,
    "has_witness":true,
    "has_refinery":true,
    "hooks": [
      {"agent":"mardi_gras/obsidian","role":"polecat","has_work":true,"title":"Fix auth service"}
    ],
    "agents": [
      {"name":"obsidian","address":"mardi_gras/obsidian","session":"mg-obsidian",
       "role":"polecat","has_work":true,
       "work_title":"Fix auth service","hook_bead":"mg-001",
       "state":"working","unread_mail":0,
       "agent_alias":"Obsidian","running":true},
      {"name":"quartz","address":"mardi_gras/quartz","session":"mg-quartz",
       "role":"polecat","has_work":true,"unread_mail":1,"state":"fix_needed",
       "agent_alias":"Quartz","work_title":"Refactor config parser","hook_bead":"mg-004",
       "first_subject":"Review failed: missing tests"},
      {"name":"matt","address":"mardi_gras/crew/matt","session":"mg-matt",
       "role":"crew","has_work":true,
       "work_title":"Add monitoring dashboards","hook_bead":"mg-003",
       "state":"working","unread_mail":2,
       "agent_alias":"Matt","running":true,
       "first_subject":"Review PR #42"},
      {"name":"witness","address":"mardi_gras/witness","session":"mg-witness",
       "role":"witness","has_work":false,"state":"idle"},
      {"name":"refinery","address":"mardi_gras/refinery","session":"mg-refinery",
       "role":"refinery","has_work":false,"state":"idle"},
      {"name":"doctor","address":"mardi_gras/dog/doctor","session":"mg-dog-doctor",
       "role":"dog","has_work":true,"state":"working","unread_mail":0,
       "work_title":"Health check patrol","running":true,
       "agent_info":"claude/haiku"},
      {"name":"reaper","address":"mardi_gras/dog/reaper","session":"mg-dog-reaper",
       "role":"dog","has_work":false,"state":"idle","unread_mail":0}
    ],
    "convoys": [
      {"id":"convoy-1","title":"Auth Feature Sprint","status":"rolling",
       "issues":["mg-001","mg-002"],"landed":["mg-006"]},
      {"id":"convoy-2","title":"Docs Cleanup","status":"preparing",
       "issues":["mg-004","mg-005"]}
    ]
  }],
  "summary": {"rig_count":1,"polecat_count":2,"crew_count":1,
    "witness_count":1,"refinery_count":1,"active_hooks":1}
}
EOF
    ;;

  vitals)
    cat <<'EOF'
Dolt Servers
  ● :13409  production  PID 15103  8.0 MB  1/1000 conn  0ms
  ○ :3307   staging     PID 9201

Databases (2 registered)
  Rig          Total  Open  Closed     %
  beads_mg        12     8       4   33%

Backups
  Local:  2026-03-01 12:00:00 (1h ago)
  JSONL:  not available
EOF
    ;;

  costs)
    cat <<'EOF'
{
  "period": "last 24h",
  "total": {"input_tokens": 245000, "output_tokens": 89000, "cost": 4.82},
  "sessions": 7,
  "by_role": [
    {"role": "polecat", "sessions": 4, "cost": 3.10},
    {"role": "crew", "sessions": 2, "cost": 1.52},
    {"role": "coordinator", "sessions": 1, "cost": 0.20}
  ],
  "by_rig": [
    {"rig": "mardi_gras", "sessions": 6, "cost": 4.62},
    {"rig": "hq", "sessions": 1, "cost": 0.20}
  ]
}
EOF
    ;;

  convoy)
    case "$2" in
      list)
        cat <<'EOF'
[
  {"id":"convoy-1","title":"Auth Feature Sprint","status":"rolling",
   "branch":"feat/auth","issues":["mg-001","mg-002"],
   "landed":["mg-006"],"created_at":"2026-02-28T09:00:00Z"},
  {"id":"convoy-2","title":"Docs Cleanup","status":"preparing",
   "branch":"chore/docs","issues":["mg-004","mg-005"],
   "landed":[],"created_at":"2026-02-28T14:00:00Z"}
]
EOF
        ;;
      *)
        echo "{}" ;;
    esac
    ;;

  mail)
    case "$2" in
      inbox)
        cat <<'EOF'
[
  {"id":"mail-1","from":"mayor","to":"crew/matt","subject":"Weekly patrol report",
   "body":"All clear this week. No incidents.","timestamp":"2026-03-01T08:00:00Z",
   "read":false},
  {"id":"mail-2","from":"obsidian","to":"crew/matt","subject":"Review PR #42",
   "body":"Auth service PR is ready for review.","timestamp":"2026-03-01T10:30:00Z",
   "read":false},
  {"id":"mail-3","from":"refinery","to":"crew/matt","subject":"Lint pass complete",
   "body":"All checks passed on feat/auth branch.","timestamp":"2026-02-28T22:00:00Z",
   "read":true}
]
EOF
        ;;
      *)
        echo "ok" ;;
    esac
    ;;

  *)
    echo "fake-gt: unknown command '$1'" >&2
    exit 1
    ;;
esac
