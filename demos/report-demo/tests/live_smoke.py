"""Real browser planning -> plugin approval -> Bus experts -> formal report download.

Runs actual model calls and user commands in an explicitly selected demo environment.
Writes evidence only below its private root. Does not inject service failures or
claim independent business acceptance. Install Playwright/browser before running.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import time

import requests
from playwright.sync_api import sync_playwright


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--private-root', type=Path, required=True)
    parser.add_argument('--web-url', default='http://127.0.0.1:5173')
    parser.add_argument('--browser-executable')
    parser.add_argument('--resume-task', help='Reconnect the browser to this smoke run; never creates another task')
    args = parser.parse_args()
    root = args.private_root.resolve()
    cfg = json.loads((root / 'config.json').read_text(encoding='utf-8'))
    evidence = root / 'evidence/platform-live-smoke.json'
    record = {'schema_version': 'report-platform-smoke/1', 'phases': [], 'checks': {},
              'limits': ['Actual local model/services; not peer independent replay or business signoff']}

    def save(phase, **values):
        record['phases'].append({'phase': phase, 'at': time.time(), **values})
        evidence.write_text(json.dumps(record, ensure_ascii=False, indent=2), encoding='utf-8')
        print(json.dumps({'phase': phase, **values}, ensure_ascii=False), flush=True)

    def get(path):
        response = requests.get(cfg['public_url'] + path,
                                headers={'Authorization': 'Bearer ' + cfg['tokens']['operator']}, timeout=15)
        response.raise_for_status()
        return response.json()

    prior = {r['plan'] for r in get('/v1/workbench/plans')['plans']}
    with sync_playwright() as pw:
        options = {'headless': True}
        if args.browser_executable:
            options['executable_path'] = args.browser_executable
        browser = pw.chromium.launch(**options)
        page = browser.new_page(viewport={'width': 1500, 'height': 1000}, accept_downloads=True)
        try:
            page.goto(args.web_url.rstrip('/') + '/chat/new')
            page.wait_for_timeout(1500)
            if args.resume_task:
                record = json.loads(evidence.read_text(encoding='utf-8'))
                if record.get('task_id') != args.resume_task or not record['checks'].get('zero_calls_before_confirmation'):
                    raise RuntimeError('Resume requires this private root previous fresh-planning smoke evidence')
                task = get('/v1/tasks/' + args.resume_task)
                save('browser_reconnected', task_id=args.resume_task)
            else:
                for label in ('专家', '我的专家', '年报协作规划助手', '使用'):
                    page.get_by_text(label, exact=True).first.click()
                    page.wait_for_timeout(500)
                editor = page.get_by_test_id('chat-panel-input')
                editor.fill('请使用年报协作技能，基于固定招商银行2025年报快照，生成本集团2025对比2024的经营与资产质量简报计划，覆盖八项指标及来源。独立资料提取、经营、资产质量专家工作后交写作专家，包含程序检查和最多一次定向修订。先真实读取目录和调用能力选择工具，再生成带选择记录的新候选图，交我确认。本次为新任务，请不要使用历史任务图。')
                editor.press('Enter')
                save('new_dialogue_submitted')
                deadline = time.monotonic() + 360
                task = None
                while time.monotonic() < deadline:
                    for row in get('/v1/workbench/plans')['plans']:
                        if row['plan'] in prior:
                            continue
                        candidate = get('/v1/tasks/' + row['plan'])
                        if candidate.get('model_binding_digest') == cfg['model_binding_digest'] and candidate['graph'].get('selection'):
                            task = candidate
                            break
                    if task:
                        break
                    page.wait_for_timeout(1000)
                if not task:
                    raise RuntimeError('No fresh selected model-authored candidate within planning deadline')
            task_id = task['task_id']
            path = '/v1/tasks/' + task_id
            record.update(task_id=task_id, graph_digest=task['graph_digest'],
                          execution_profile=task['execution_profile'], model_binding_digest=task['model_binding_digest'])
            if not args.resume_task:
                record['session_url'] = page.url
                record['checks']['zero_calls_before_confirmation'] = task['status'] == 'AWAITING_APPROVAL' and get(path + '/calls') == []
            save('reconnected_task' if args.resume_task else 'fresh_candidate', task_id=task_id, nodes=len(task['graph']['nodes']))

            page.get_by_role('button', name='编排台', exact=True).click()
            frame = page.frame_locator('iframe')
            frame.locator('.wb-item', has_text=task_id).click()
            if not args.resume_task:
                frame.locator('.cg-head button').click()
                frame.locator('.cg-gate input[type=password]').fill(cfg['user_password'])
                frame.locator('.cg-gate button[type=submit]').click()
                save('plugin_admission_submitted')
            deadline = time.monotonic() + 900
            last_status = None
            while time.monotonic() < deadline:
                task = get(path)
                if task['status'] != last_status:
                    save('task_status', status=task['status'])
                    last_status = task['status']
                if task['status'] in ('AWAITING_ACCEPTANCE', 'COMPUTE_COMPLETED'):
                    break
                if task['status'] in ('FAILED', 'NEEDS_REVIEW', 'BLOCKED'):
                    raise RuntimeError('Real task requires attention: ' + task['status'])
                page.wait_for_timeout(1000)
            if task['status'] not in ('AWAITING_ACCEPTANCE', 'COMPUTE_COMPLETED'):
                raise RuntimeError('Real experts did not settle within execution deadline')
            calls = get(path + '/calls')
            record['checks']['four_real_experts'] = {c['agent_id'] for c in calls if c['state'] == 'COMPLETED'} == {
                'report-extractor', 'business-analyst', 'asset-quality-analyst', 'report-writer'}
            record['checks']['all_calls_have_unique_completed_remote_tasks'] = all(
                c.get('remote_task_id') and c['state'] == 'COMPLETED' and not c['conflicted'] for c in calls
            ) and len({c['remote_task_id'] for c in calls}) == len(calls)

            frame.locator('.wb-item', has_text=task_id).click()
            page.wait_for_timeout(1500)
            qualified = [r for r in get('/v1/workbench/tasks/' + task_id)['reports'] if r['can_approve']]
            if not qualified:
                raise RuntimeError('No report qualified for approval; do not bypass HUMAN/FAIL')
            detail = frame.locator('details[data-artifact="' + qualified[-1]['artifact_id'] + '"]')
            detail.locator('summary').click()
            frame.locator('input[type=password]').last.fill(cfg['user_password'])
            detail.get_by_role('button', name='批准此报告', exact=True).click()
            page.wait_for_timeout(1000)
            frame.locator('input[type=password]').last.fill(cfg['user_password'])
            detail.get_by_role('button', name='交付此报告', exact=True).click()
            deadline = time.monotonic() + 30
            while time.monotonic() < deadline:
                task = get(path)
                if task['status'] == 'DELIVERED':
                    break
                page.wait_for_timeout(500)
            record['checks']['formally_delivered'] = task['status'] == 'DELIVERED' and bool(task.get('delivery_receipts'))
            projection = get('/v1/workbench/tasks/' + task_id)
            reports = {r['artifact_id']: r for r in projection['reports']}
            artifact = detail.get_attribute('data-artifact')
            if artifact not in reports:
                raise RuntimeError('Plugin report does not match authority projection')
            frame.locator('input[type=password]').last.fill(cfg['user_password'])
            with page.expect_download() as pending:
                detail.get_by_role('button', name='下载正式 JSON', exact=True).click()
            raw = Path(pending.value.path()).read_bytes()
            digest = 'sha256:' + hashlib.sha256(raw).hexdigest()
            record['checks']['download_exact_formal_bytes'] = digest == reports[artifact]['digest']
            record['checks']['evidence_intact'] = get(path + '/verify')['evidence_intact']
            record.update(report_digest=digest, remote_calls=len(calls), status=task['status'])
            record['all_passed'] = all(record['checks'].values())
            page.screenshot(path=str(root / 'evidence/platform-smoke-delivered.png'))
            save('completed', task_id=task_id, status=task['status'], all_passed=record['all_passed'])
            if not record['all_passed']:
                raise RuntimeError('A real-service assertion failed')
        except Exception:
            page.screenshot(path=str(root / 'evidence/platform-smoke-failed.png'))
            save('failed')
            raise
        finally:
            browser.close()


if __name__ == '__main__':
    main()
