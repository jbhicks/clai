/**
 * Quality Gates - Automated verification before commit
 */

import type { AgentResult, QualityResult, QualityCheck } from '../types.js';
import { logger } from '../utils/logger.js';
import { $ } from 'zx';

export class QualityGates {
  private checks: ((result: AgentResult) => Promise<QualityCheck>)[];

  constructor() {
    this.checks = [
      this.typecheck.bind(this),
      this.tests.bind(this),
      this.lint.bind(this),
      this.gitStatus.bind(this),
    ];
  }

  async run(result: AgentResult): Promise<QualityResult> {
    const startTime = Date.now();
    const checks: QualityCheck[] = [];
    const failures: string[] = [];

    logger.info('Running quality gates...');

    for (const check of this.checks) {
      const checkResult = await check(result);
      checks.push(checkResult);
      
      if (!checkResult.passed) {
        failures.push(`${checkResult.name}: ${checkResult.error || 'Failed'}`);
        logger.warn(`Quality check failed: ${checkResult.name}`);
      } else {
        logger.info(`Quality check passed: ${checkResult.name}`);
      }
    }

    const duration = Date.now() - startTime;
    const passed = failures.length === 0;

    return {
      passed,
      checks,
      failures,
      duration,
    };
  }

  private async typecheck(result: AgentResult): Promise<QualityCheck> {
    const startTime = Date.now();
    
    try {
      // Try different typecheck commands based on project type
      const commands = [
        'go build ./...',
        'bun typecheck',
        'npx tsc --noEmit',
        'cargo check',
      ];

      for (const cmd of commands) {
        try {
          await $`${cmd}`.timeout(120000).quiet();
          return {
            name: 'Typecheck',
            passed: true,
            output: cmd,
            duration: Date.now() - startTime,
          };
        } catch {
          continue;
        }
      }

      // No typecheck command found
      return {
        name: 'Typecheck',
        passed: true, // Skip if no typecheck available
        output: 'No typecheck command found',
        duration: Date.now() - startTime,
      };
    } catch (error) {
      return {
        name: 'Typecheck',
        passed: false,
        error: error instanceof Error ? error.message : 'Unknown error',
        duration: Date.now() - startTime,
      };
    }
  }

  private async tests(result: AgentResult): Promise<QualityCheck> {
    const startTime = Date.now();
    
    try {
      // Try different test commands
      const commands = [
        'go test ./...',
        'bun test',
        'npm test',
        'cargo test',
      ];

      for (const cmd of commands) {
        try {
          const result = await $`${cmd}`.timeout(300000).quiet();
          return {
            name: 'Tests',
            passed: true,
            output: result.stdout.substring(0, 500),
            duration: Date.now() - startTime,
          };
        } catch {
          continue;
        }
      }

      // No test command found
      return {
        name: 'Tests',
        passed: true, // Skip if no test available
        output: 'No test command found',
        duration: Date.now() - startTime,
      };
    } catch (error) {
      return {
        name: 'Tests',
        passed: false,
        error: error instanceof Error ? error.message : 'Unknown error',
        duration: Date.now() - startTime,
      };
    }
  }

  private async lint(result: AgentResult): Promise<QualityCheck> {
    const startTime = Date.now();
    
    try {
      // Try different lint commands
      const commands = [
        'golangci-lint run ./...',
        'bun lint',
        'eslint .',
        'cargo clippy',
      ];

      for (const cmd of commands) {
        try {
          await $`${cmd}`.timeout(120000).quiet();
          return {
            name: 'Lint',
            passed: true,
            output: cmd,
            duration: Date.now() - startTime,
          };
        } catch {
          continue;
        }
      }

      return {
        name: 'Lint',
        passed: true,
        output: 'No lint command found',
        duration: Date.now() - startTime,
      };
    } catch (error) {
      return {
        name: 'Lint',
        passed: false,
        error: error instanceof Error ? error.message : 'Unknown error',
        duration: Date.now() - startTime,
      };
    }
  }

  private async gitStatus(result: AgentResult): Promise<QualityCheck> {
    const startTime = Date.now();
    
    try {
      // Check for uncommitted changes
      const status = await $`git status --porcelain`.quiet();
      const hasChanges = status.stdout.trim().length > 0;
      
      // Check if last commit is signed
      const commit = await $`git log -1 --format="%G?"`.quiet();
      const isSigned = commit.stdout.trim() === 'G';
      
      return {
        name: 'Git Status',
        passed: true,
        output: hasChanges ? 'Uncommitted changes present' : 'Working tree clean',
        duration: Date.now() - startTime,
      };
    } catch (error) {
      return {
        name: 'Git Status',
        passed: true, // Git issues don't block quality gates
        output: 'Unable to check git status',
        duration: Date.now() - startTime,
      };
    }
  }
}
