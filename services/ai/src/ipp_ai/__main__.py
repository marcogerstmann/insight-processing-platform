"""Local entrypoint for `make ai-run-local`.

No inbound adapter exists yet — AI 4 (IPP-95) adds the EventBridge/SQS
handler this will eventually delegate to, the same way cmd/*-local pairs
with cmd/*-lambda for the Go services. This only proves the scaffold
(venv, package layout, imports) actually runs.
"""

from __future__ import annotations


def main() -> None:
    print("ipp-ai scaffold OK — no inbound handler wired yet (see IPP-95).")


if __name__ == "__main__":
    main()
