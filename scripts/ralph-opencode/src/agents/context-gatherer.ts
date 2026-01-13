/**
 * Context Gatherer - Parallel background task execution
 * 
 * OhMyOpenCode superpower: gather context in parallel before main task
 */

import type { GatheredContext, Story, Pattern, CodebaseInfo } from '../types.js';
import { logger } from '../utils/logger.js';
import { $ } from 'zx';

export interface ContextGathererConfig {
  parallelAgents?: boolean;
  timeout?: number;
}

export class ContextGatherer {
  private config: Required<ContextGathererConfig>;

  constructor(config: ContextGathererConfig = {}) {
    this.config = {
      parallelAgents: config.parallelAgents ?? true,
      timeout: config.timeout || 30000, // 30 seconds
    };
  }

  async gather(params: {
    story: Story;
    stories: Story[];
    agentsMdPath: string;
  }): Promise<GatheredContext> {
    const { story, stories, agentsMdPath } = params;
    
    logger.debug(`Gathering context for: ${story.title}`);
    
    // Run all gatherers in parallel (OhMyOpenCode superpower!)
    const [patterns, codebaseInfo, relevantAgentsMd] = await Promise.all([
      this.gatherPatterns(stories),
      this.gatherCodebaseInfo(),
      this.readAgentsMd(agentsMdPath),
    ]);

    return {
      story,
      patterns,
      codebaseInfo,
      relevantAgentsMd,
    };
  }

  private async gatherPatterns(stories: Story[]): Promise<Pattern[]> {
    // Look for patterns in previous stories and AGENTS.md
    const patterns: Pattern[] = [];
    
    // Find completed stories and extract their patterns
    for (const story of stories) {
      if (story.passes && story.notes) {
        patterns.push({
          name: story.id,
          description: story.title,
          context: story.description,
          source: 'previous_story',
        });
      }
    }

    return patterns;
  }

  private async gatherCodebaseInfo(): Promise<CodebaseInfo> {
    // Detect language and build commands
    const language = await this.detectLanguage();
    const buildCommand = this.getBuildCommand(language);
    const testCommand = this.getTestCommand(language);
    const keyFiles = await this.findKeyFiles(language);
    const patterns = await this.findCodebasePatterns();

    return {
      language,
      buildCommand,
      testCommand,
      keyFiles,
      patterns,
    };
  }

  private async detectLanguage(): Promise<string> {
    // Check for common language indicators
    const checks = [
      { pattern: '**/*.go', lang: 'go' },
      { pattern: '**/*.ts', lang: 'typescript' },
      { pattern: '**/*.py', lang: 'python' },
      { pattern: '**/*.rs', lang: 'rust' },
    ];

    for (const check of checks) {
      try {
        const files = await $`find . -maxdepth 2 -name ${check.pattern} -type f`.quiet();
        if (files.stdout.trim()) {
          return check.lang;
        }
      } catch {
        continue;
      }
    }

    return 'unknown';
  }

  private getBuildCommand(language: string): string {
    const commands: Record<string, string> = {
      go: 'go build ./...',
      typescript: 'bun build',
      python: 'python -m py_compile',
      rust: 'cargo check',
    };

    return commands[language] || 'echo "No build command"';
  }

  private getTestCommand(language: string): string {
    const commands: Record<string, string> = {
      go: 'go test ./...',
      typescript: 'bun test',
      python: 'python -m pytest',
      rust: 'cargo test',
    };

    return commands[language] || 'echo "No test command"';
  }

  private async findKeyFiles(language: string): Promise<string[]> {
    const keyFiles: Record<string, string[]> = {
      go: ['main.go', 'go.mod', 'Makefile'],
      typescript: ['package.json', 'tsconfig.json', 'index.ts'],
      python: ['main.py', 'requirements.txt', 'pyproject.toml'],
      rust: ['Cargo.toml', 'src/main.rs'],
    };

    const candidates = keyFiles[language] || [];
    const found: string[] = [];

    for (const file of candidates) {
      try {
        await $`test -f ${file}`.quiet();
        found.push(file);
      } catch {
        continue;
      }
    }

    return found;
  }

  private async findCodebasePatterns(): Promise<string[]> {
    // Look for common patterns in the codebase
    const patterns: string[] = [];
    
    try {
      // Check for specific patterns based on file existence
      const hasTests = await $`find . -name "*_test.go" -o -name "*.test.ts" -o -name "test_*.py"`.quiet();
      if (hasTests.stdout.trim()) {
        patterns.push('has_tests');
      }

      const hasDocker = await $`test -f Dockerfile`.quiet();
      if (hasDocker) {
        patterns.push('has_docker');
      }

      const hasGitHubActions = await $`test -d .github/workflows`.quiet();
      if (hasGitHubActions) {
        patterns.push('has_ci');
      }
    } catch {
      // Ignore errors
    }

    return patterns;
  }

  private async readAgentsMd(path: string): Promise<string | undefined> {
    try {
      return await $`cat ${path}`.quiet().then(r => r.stdout);
    } catch {
      return undefined;
    }
  }
}
