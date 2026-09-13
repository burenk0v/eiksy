from pathlib import Path

SERVICE = Path('internal/app/service.go')
POLICY = Path('internal/app/command_policy.go')
WORKFLOW = Path('.github/workflows/apply-command-policy-v2-integration.yml')


def main():
    service = SERVICE.read_text()
    marker = 'func commandActionForRequest('
    start = service.find(marker)
    if start >= 0:
        end_marker = 'func commandAllowedByPolicy('
        end = service.find(end_marker, start)
        if end < 0:
            raise SystemExit('commandAllowedByPolicy marker not found')
        service = service[:start] + service[end:]
        SERVICE.write_text(service)

    policy = POLICY.read_text()
    if 'func commandActionForRequest(' not in policy:
        marker = 'func evaluateCommandPolicy('
        idx = policy.find(marker)
        if idx < 0:
            raise SystemExit('evaluateCommandPolicy marker not found')
        wrapper = '''func commandActionForRequest(policy ai.CommandPolicy, toolID, sessionID, command string) ai.CommandPermissionAction {
\tdecision, _ := evaluateCommandPolicy(policy, toolID, sessionID, command)
\tswitch decision {
\tcase commandPolicyDecisionAllow:
\t\treturn ai.CommandPermissionAllow
\tcase commandPolicyDecisionDeny:
\t\treturn ai.CommandPermissionDeny
\tdefault:
\t\treturn ai.CommandPermissionAsk
\t}
}

'''
        policy = policy[:idx] + wrapper + policy[idx:]
        POLICY.write_text(policy)

    if WORKFLOW.exists():
        WORKFLOW.unlink()


if __name__ == '__main__':
    main()
