import type { Graph } from "../types";

export function HumanReviewNotice({review}: {review: Graph["human_review"]}) {
  if (!review?.checks.length) return null;
  return <div role="status">
    <p>检查需要人工核验（HUMAN）。请在 Go 可信处理页登录、核对报告与检查依据，再作出判断。</p>
    <a href={review.url} target="_blank" rel="noopener noreferrer">前往人工处理</a>
    <p>处理后返回编排台刷新状态；人工接受会保留原始 HUMAN 检查记录。</p>
  </div>;
}
