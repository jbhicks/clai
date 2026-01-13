/**
 * Configuration loader
 */

import { promises as fs } from 'fs';
import { join } from 'path';

export interface Config {
  storiesFile?: string;
  progressFile?: string;
  agentsMdFile?: string;
  maxIterations?: number;
  verbose?: boolean;
  model?: string;
  branchName?: string;
  autoHandoff?: boolean;
}

export async function loadConfig(configPath?: string): Promise<Config> {
  const paths = configPath 
    ? [configPath]
    : [
        '.ralphrc.json',
        '.ralphrc',
        'ralph.config.json',
        './.clai/ralph.config.json',
      ];

  for (const path of paths) {
    try {
      const content = await fs.readFile(path, 'utf-8');
      const config = JSON.parse(content);
      
      // Apply defaults
      return {
        storiesFile: config.storiesFile || './.clai/stories.json',
        progressFile: config.progressFile || './.clai/progress.json',
        agentsMdFile: config.agentsMdFile || './AGENTS.md',
        maxIterations: config.maxIterations || 50,
        verbose: config.verbose || false,
        model: config.model || 'opencode/claude-opus-4-5',
        branchName: config.branchName || 'ralph/feature',
        autoHandoff: config.autoHandoff ?? true,
      };
    } catch {
      continue;
    }
  }

  // Default config
  return {
    storiesFile: './.clai/stories.json',
    progressFile: './.clai/progress.json',
    agentsMdFile: './AGENTS.md',
    maxIterations: 50,
    verbose: false,
    model: 'opencode/claude-opus-4-5',
    branchName: 'ralph/feature',
    autoHandoff: true,
  };
}
