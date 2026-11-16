// Shared by the embedded graph card and trusted user page. No state authority.
class ReportProjection {
  constructor(taskId = null) { this.taskId = taskId; this.generation = null; this.commands = new Map(); }
  selectTask(taskId) {
    if (this.taskId !== taskId) { this.taskId = taskId; this.generation = null; }
  }
  accept(task) {
    if (!task || task.task_id !== this.taskId || typeof task.generation !== 'string' || !/^\d+$/.test(task.generation)) return false;
    const generation = BigInt(task.generation);
    if (this.generation !== null && generation < this.generation) return false;
    this.generation = generation;
    return true;
  }
  runCommand(key, command) {
    if (this.commands.has(key)) return this.commands.get(key);
    const pending = Promise.resolve().then(command).finally(() => this.commands.delete(key));
    this.commands.set(key, pending);
    return pending;
  }
}
