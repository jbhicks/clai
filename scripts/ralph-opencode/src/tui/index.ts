/**
 * Beautiful Terminal UI for Ralph Loop
 */

import type { Progress, CommitInfo, FailureInfo, GatheredContext } from '../types.js';
import ora from 'ora';
import cliProgress from 'cli-progress';
import figures from 'figures';
import { logger } from '../utils/logger.js';

export class TUI {
  private spinner: ora.Ora | null = null;
  private progressBar: cliProgress.SingleBar | null = null;
  private logs: string[] = [];
  private commits: CommitInfo[] = [];
  private failures: FailureInfo[] = [];
  private currentStatus = 'idle';
  private context: GatheredContext | null = null;

  async initialize(options: { totalStories: number; branchName: string }): Promise<void> {
    console.clear();
    this.printHeader(options);
    this.initProgressBar(options.totalStories);
    this.log(`Initialized Ralph loop on branch: ${options.branchName}`);
    this.log(`Ready to execute ${options.totalStories} stories`);
  }

  private printHeader(options: { totalStories: number; branchName: string }): void {
    console.log(`
╔══════════════════════════════════════════════════════════════════╗
║                                                                  ║
║   🏔️  R A L P H   L O O P                                        ║
║   ─────────────────────────────────────────────────────────────  ║
║   Autonomous Development with OhMyOpenCode                       ║
║                                                                  ║
║   📚 Stories: ${options.totalStories.toString().padEnd(8)}🌿 Branch: ${options.branchName}           ║
║                                                                  ║
╚══════════════════════════════════════════════════════════════════╝
`);
  }

  private initProgressBar(total: number): void {
    this.progressBar = new cliProgress.SingleBar({
      format: '📊 Progress [{bar}] {percentage}% | {value}/{total} stories',
      barCompleteChar: '█',
      barIncompleteChar: '░',
      hideCursor: true,
    }, cliProgress.Presets.shades_classic);
    
    this.progressBar.start(total, 0);
  }

  updateIteration(iteration: number, progress: Progress): void {
    this.progressBar?.update(progress.completed);
    
    console.log(`
${figures.arrowRight} Iteration ${iteration}
${figures.arrowRight} Progress: ${progress.completed}/${progress.total} (${progress.percentage}%)
`);
  }

  setStatus(status: 'idle' | 'executing' | 'paused'): void {
    this.currentStatus = status;
    const statusText = {
      idle: 'Waiting for task...',
      executing: '🤖 Agent executing...',
      paused: '⏸️  Paused',
    };
    
    this.spinner?.stop();
    this.spinner = ora({
      text: statusText[status],
      spinner: 'dots',
    }).start();
  }

  log(message: string, level: 'info' | 'warn' | 'error' = 'info'): void {
    const prefix = {
      info: figures.info,
      warn: figures.warning,
      error: figures.cross,
    };
    
    const timestamp = new Date().toLocaleTimeString();
    const logLine = `[${timestamp}] ${prefix[level]} ${message}`;
    
    this.logs.push(logLine);
    
    // Keep only last 100 logs
    if (this.logs.length > 100) {
      this.logs = this.logs.slice(-100);
    }
    
    console.log(logLine);
  }

  addCommit(commit: CommitInfo): void {
    this.commits.push(commit);
    const timestamp = new Date().toLocaleTimeString();
    console.log(`
✅ Commit: ${commit.id}
   ${commit.title}
   Files: ${commit.files.join(', ') || 'N/A'}
   ${figures.clock} ${timestamp}
`);
  }

  addFailure(failure: FailureInfo): void {
    this.failures.push(failure);
    const timestamp = new Date().toLocaleTimeString();
    console.log(`
❌ Failed: ${failure.storyId}
   ${failure.reason}
   ${figures.clock} ${timestamp}
`);
  }

  setContext(context: GatheredContext): void {
    this.context = context;
    
    console.log(`
📋 Context Gathered:
   Language: ${context.codebaseInfo.language}
   Key Files: ${context.codebaseInfo.keyFiles.slice(0, 3).join(', ')}
   Patterns: ${context.codebaseInfo.patterns.join(', ') || 'None'}
`);
  }

  async showPaused(): Promise<void> {
    this.spinner?.stop();
    console.log(`
⏸️  PAUSED
   Press Enter to resume, or Ctrl+C to quit
`);
    
    // Wait for input
    await new Promise(resolve => {
      process.stdin.once('data', resolve);
    });
  }

  async showSummary(data: {
    success: boolean;
    iterations: number;
    completedStories: number;
    totalStories: number;
    duration: number;
  }): Promise<void> {
    this.spinner?.stop();
    this.progressBar?.stop();
    
    const duration = this.formatDuration(data.duration);
    const successRate = Math.round((data.completedStories / data.totalStories) * 100);
    
    console.log(`
╔══════════════════════════════════════════════════════════════════╗
║                     R A L P H   S U M M A R Y                     ║
╠══════════════════════════════════════════════════════════════════╣
║                                                                  ║
║   ${data.success ? '🎉 COMPLETED!' : '📊 PARTIAL'}                                            ║
║                                                                  ║
║   Iterations:     ${data.iterations.toString().padEnd(8)}   Duration:    ${duration.padEnd(8)}     ║
║   Completed:      ${data.completedStories.toString().padEnd(8)}   Total:       ${data.totalStories.toString().padEnd(8)}      ║
║   Success Rate:   ${successRate.toString().padEnd(8)}%                                           ║
║                                                                  ║
║   Commits:        ${this.commits.length.toString().padEnd(8)}   Failures:    ${this.failures.length.toString().padEnd(8)}      ║
║                                                                  ║
╚══════════════════════════════════════════════════════════════════╝
`);
    
    if (this.commits.length > 0) {
      console.log('\n📦 Recent Commits:');
      for (const commit of this.commits.slice(-5)) {
        console.log(`   • ${commit.id}: ${commit.title}`);
      }
    }
    
    if (this.failures.length > 0) {
      console.log('\n⚠️  Failures:');
      for (const failure of this.failures) {
        console.log(`   • ${failure.storyId}: ${failure.reason}`);
      }
    }
  }

  private formatDuration(ms: number): string {
    const seconds = Math.floor(ms / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    
    if (hours > 0) {
      return `${hours}h ${minutes % 60}m`;
    } else if (minutes > 0) {
      return `${minutes}m ${seconds % 60}s`;
    } else {
      return `${seconds}s`;
    }
  }
}
