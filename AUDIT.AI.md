# Alignment audit findings

Temporary audit record. Delete with the alignment commit after all listed findings pass validation.

1. GitHub lacked beta and devel workflows. Added both from AI.md PART 27.
2. GitHub Docker triggers and beta/devel tag selection differed from AI.md. Replaced the workflow from the template and corrected channel rules.
3. Gitea and Forgejo lacked beta, devel, and Docker workflows. Added provider-equivalent files from AI.md PART 27.
4. Gitea and Forgejo stable release workflows omitted the default static CLI artifacts. Added those artifacts.
5. GitLab used an overbroad platform build matrix and lacked beta/devel release and Docker channel jobs. Replaced it with the AI.md channel-specific jobs and current-platform compile gate.
6. GitLab scheduled devel release jobs depended on a test job omitted from scheduled pipelines. Added the schedule rule to the test job; recorded the matching AI.md template contradiction in the shared TODO.API.md inbox.
7. GitLab third-party images used floating tags. Pinned them to registry digests.
8. GitLab release entries omitted static CLI and GUI artifact links. Added links and corrected stable artifact paths to include the corresponding outputs.
9. TODO.AI.md had completed items checked off instead of being cleared in the resolving commit. Clear completed items and the finished plan in the alignment commit per AI.md TODO Hygiene.
