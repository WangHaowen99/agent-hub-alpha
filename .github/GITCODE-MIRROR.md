# Agent Hub Alpha cross-platform collaboration

GitHub `chaosxingxc-orion/agent-hub-alpha` is the final integration repository.
GitCode `xingchao_ml/agent-hub-alpha` is the corresponding team workspace.
NGA retains model experiments; this bridge never copies code between the two projects.

GitHub main, github/** branches and v* tags propagate by ordinary fast-forward
pushes. GitCode requests import into gitcode/mr-N; GitHub-native requests project
to github/pr-N. These namespaces are bridge-owned. Developers use other branches.
No automatic approval, merge, force push, branch deletion or tag replacement.
Divergence fails visibly and must be resolved with history preserved.

Comments retain author and source links. Reserved nga-bridge markers prevent
loops and are scoped to this repository pair. Edits update copies; deletions,
labels, assignees, releases, attachments and LFS are outside this bridge.

GitHub events trigger reconciliation. GitCode changes are polled every five
minutes; provider delays mean eventual consistency, not a real-time SLA.
Use workflow_dispatch for immediate catch-up. Runs are serialized, use trusted
main only and never execute imported code. GITCODE_TOKEN is a repository secret;
GitHub uses its per-run token. Never put credentials in comments.

Run both test_sync_gitcode.py and test_collaboration_bridge.py before changing
the bridge. Inspect authentication, divergence or API failures before rerunning.
Do not start a competing reverse full mirror. Generated requests start as drafts;
copied reviews are discussion, not approval.

[Migration and historical discussion](../docs/migration/README.md).
