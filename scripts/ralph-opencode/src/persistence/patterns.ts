/**
 * Pattern Learner - Discover and persist patterns in AGENTS.md
 */

import { logger } from '../utils/logger.js';
import { promises as fs } from 'fs';
import { join, dirname } from 'path';

export interface PatternLearnerConfig {
  maxPatterns?: number;
  compactThreshold?: number;
}

export interface Pattern {
  name: string;
  description: string;
  context: string;
  source: string;
  timestamp?: string;
}

export class PatternLearner {
  private agentsMdPath: string;
  private maxPatterns: number;
  private compactThreshold: number;

  constructor(agentsMdPath: string, config: PatternLearnerConfig = {}) {
    this.agentsMdPath = agentsMdPath;
    this.maxPatterns = config.maxPatterns || 50;
    this.compactThreshold = config.compactThreshold || 100;
  }

  async learn(pattern: Pattern): Promise<void> {
    try {
      // Read current AGENTS.md
      let content = '';
      try {
        content = await fs.readFile(this.agentsMdPath, 'utf-8');
      } catch {
        // File doesn't exist, create it
        content = this.createNewAgentsMd();
      }

      // Check if pattern already exists
      if (this.patternExists(content, pattern)) {
        logger.debug(`Pattern already exists: ${pattern.name}`);
        return;
      }

      // Add pattern to AGENTS.md
      const newContent = this.addPattern(content, pattern);
      await fs.writeFile(this.agentsMdPath, newContent, 'utf-8');
      
      logger.info(`Learned pattern: ${pattern.name}`);
    } catch (error) {
      logger.error(`Failed to learn pattern: ${pattern.name}`, error);
    }
  }

  private patternExists(content: string, pattern: Pattern): boolean {
    // Check if pattern with same name exists
    const patternSection = content.split('## Codebase Patterns')[1] || '';
    return patternSection.includes(`**${pattern.name}**`);
  }

  private addPattern(content: string, pattern: Pattern): string {
    const patternEntry = this.formatPattern(pattern);
    
    // Find or create Codebase Patterns section
    if (content.includes('## Codebase Patterns')) {
      // Add to existing section
      const parts = content.split('## Codebase Patterns');
      const before = parts[0];
      const after = parts.slice(1).join('## Codebase Patterns');
      
      // Add before the first subheader in the patterns section
      const afterParts = after.split('\n## ');
      const patternsContent = patternEntry + '\n' + afterParts[0];
      const remaining = afterParts.slice(1).join('\n## ');
      
      return before + '## Codebase Patterns\n' + patternsContent + '\n## ' + remaining;
    } else {
      // Add new section
      return content + '\n\n## Codebase Patterns\n' + patternEntry;
    }
  }

  private formatPattern(pattern: Pattern): string {
    return `
### **${pattern.name}**
${pattern.description}

${pattern.context}
`.trim();
  }

  private createNewAgentsMd(): string {
    return `# AGENTS.md

This file contains patterns discovered by Ralph autonomous agent loop.

## Codebase Patterns

`;
  }

  async compact(): Promise<void> {
    try {
      const content = await fs.readFile(this.agentsMdPath, 'utf-8');
      const patternSection = content.split('## Codebase Patterns')[1];
      
      if (!patternSection) return;
      
      // Count patterns
      const patternCount = (patternSection.match(/^### \*\*/gm) || []).length;
      
      if (patternCount < this.compactThreshold) return;
      
      logger.info(`Compacting AGENTS.md (${patternCount} patterns)...`);
      
      // Keep only the most recent patterns
      const lines = patternSection.split('\n');
      const patterns: string[] = [];
      let currentPattern: string[] = [];
      
      for (const line of lines) {
        if (line.startsWith('### **')) {
          if (currentPattern.length > 0) {
            patterns.push(currentPattern.join('\n'));
          }
          currentPattern = [line];
        } else if (line.trim() && currentPattern.length > 0) {
          currentPattern.push(line);
        }
      }
      
      if (currentPattern.length > 0) {
        patterns.push(currentPattern.join('\n'));
      }
      
      // Keep only the last N patterns
      const keptPatterns = patterns.slice(-this.maxPatterns);
      const newPatternSection = keptPatterns.join('\n\n');
      
      const newContent = content.split('## Codebase Patterns')[0] + 
        '## Codebase Patterns\n\n' + newPatternSection;
      
      await fs.writeFile(this.agentsMdPath, newContent, 'utf-8');
      
      logger.info(`Compacted AGENTS.md to ${keptPatterns.length} patterns`);
    } catch (error) {
      logger.error('Failed to compact AGENTS.md', error);
    }
  }

  async search(query: string): Promise<Pattern[]> {
    try {
      const content = await fs.readFile(this.agentsMdPath, 'utf-8');
      const patternSection = content.split('## Codebase Patterns')[1];
      
      if (!patternSection) return [];
      
      const patterns: Pattern[] = [];
      const lines = patternSection.split('\n');
      let currentPattern: string | null = null;
      
      for (const line of lines) {
        if (line.startsWith('### **')) {
          if (currentPattern) {
            patterns.push(this.parsePattern(currentPattern));
          }
          currentPattern = line;
        } else if (currentPattern && line.trim()) {
          currentPattern += '\n' + line;
        }
      }
      
      if (currentPattern) {
        patterns.push(this.parsePattern(currentPattern));
      }
      
      // Filter by query
      return patterns.filter(p => 
        p.name.toLowerCase().includes(query.toLowerCase()) ||
        p.description.toLowerCase().includes(query.toLowerCase())
      );
    } catch {
      return [];
    }
  }

  private parsePattern(content: string): Pattern {
    const lines = content.split('\n');
    const name = lines[0].replace('### **', '').replace('**', '').trim();
    const description = lines[1]?.trim() || '';
    const context = lines.slice(2).join('\n').trim();
    
    return {
      name,
      description,
      context,
      source: 'AGENTS.md',
    };
  }
}
