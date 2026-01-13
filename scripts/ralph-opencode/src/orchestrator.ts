/**
 * RalphLoop Orchestrator - The heart of autonomous development
 * 
 * Coordinates multi-agent execution with quality gates and beautiful TUI
 */

import type { Story, IterationResult, LoopConfig, AgentResult } from './types.js';
import { OpenCodeAgent } from './agents/opencode.js';
import { ContextGatherer } from './agents/context-gatherer.js';
import { QualityGates } from './quality/gates.js';
import { PatternLearner } from './persistence/patterns.js';
import { TUI } from './tui/index.js';
import { logger } from './utils/logger.js';
import { promises as fs } from 'fs';
import { basename, join } from 'path';

export class RalphLoop {
  private config: Required<LoopConfig>;
  private tui: TUI;
  private agent: OpenCodeAgent;
  private contextGatherer: ContextGatherer;
  private qualityGates: QualityGates;
  private patternLearner: PatternLearner;
  private stories: Story[];
  private currentIteration = 0;
  private startTime: number;
  private isPaused = false;
  private isComplete = false;

  constructor(config: LoopConfig) {
    this.config = {
      storiesFile: config.storiesFile || './.clai/stories.json',
      progressFile: config.progressFile || './.clai/progress.json',
      agentsMdFile: config.agentsMdFile || './AGENTS.md',
      maxIterations: config.maxIterations || 50,
      verbose: config.verbose || false,
      model: config.model || 'opencode/claude-opus-4-5',
      branchName: config.branchName || 'ralph/feature',
      autoHandoff: config.autoHandoff ?? true,
    };

    this.tui = new TUI();
    this.agent = new OpenCodeAgent({ model: this.config.model });
    this.contextGatherer = new ContextGatherer();
    this.qualityGates = new QualityGates();
    this.patternLearner = new PatternLearner(this.config.agentsMdFile);
    
    this.stories = [];
    this.startTime = Date.now();
  }

  async run(): Promise<IterationResult> {
    try {
      // Initialize
      await this.initialize();
      
      // Main loop
      while (!this.isComplete && this.currentIteration < this.config.maxIterations) {
        if (this.isPaused) {
          await this.tui.showPaused();
          continue;
        }

        this.currentIteration++;
        
        // Get next story
        const story = this.getNextStory();
        if (!story) {
          this.isComplete = true;
          break;
        }

        // Update TUI with current iteration
        this.tui.updateIteration(this.currentIteration, this.getProgress());
        
        // Parallel context gathering (OhMyOpenCode superpower!)
        await this.gatherContext(story);
        
        // Execute the story
        const result = await this.executeStory(story);
        
        // Run quality gates
        const qualityResult = await this.qualityGates.run(result);
        
        if (qualityResult.passed) {
          // Commit and update
          await this.commitStory(story, result);
          
          // Learn patterns
          await this.learnPatterns(result);
          
          // Update progress
          this.updateStoryStatus(story.id, true);
        } else {
          // Log failure and continue
          await this.handleFailure(story, qualityResult);
        }

        // Check if all stories complete
        this.isComplete = this.areAllComplete();
        
        // Brief pause between iterations
        await this.sleep(1000);
      }

      // Final summary
      return this.summarize();
      
    } catch (error) {
      logger.error('Loop error:', error);
      return {
        success: false,
        iterations: this.currentIteration,
        completedStories: this.stories.filter(s => s.passes).length,
        totalStories: this.stories.length,
        error: error instanceof Error ? error.message : 'Unknown error',
      };
    }
  }

  private async initialize(): Promise<void> {
    logger.info('Initializing Ralph loop...');
    
    // Load stories
    await this.loadStories();
    
    // Initialize TUI
    await this.tui.initialize({
      totalStories: this.stories.length,
      branchName: this.config.branchName,
    });
    
    logger.info(`Loaded ${this.stories.length} stories`);
    this.tui.log(`Loaded ${this.stories.length} stories from ${basename(this.config.storiesFile)}`);
  }

  private async loadStories(): Promise<void> {
    try {
      const content = await fs.readFile(this.config.storiesFile, 'utf-8');
      const data = JSON.parse(content);
      this.stories = data.stories || [];
      
      // Sort by priority and phase
      this.stories.sort((a: Story, b: Story) => {
        const priorityOrder = { high: 0, medium: 1, low: 2, P0: 0, P1: 1, P2: 2, P3: 3 };
        const aPriority = priorityOrder[a.priority as keyof typeof priorityOrder] ?? 99;
        const bPriority = priorityOrder[b.priority as keyof typeof priorityOrder] ?? 99;
        return aPriority - bPriority;
      });
    } catch (error) {
      throw new Error(`Failed to load stories: ${this.config.storiesFile}`);
    }
  }

  private getNextStory(): Story | null {
    return this.stories.find(s => !s.passes) || null;
  }

  private getProgress(): { completed: number; total: number; percentage: number } {
    const completed = this.stories.filter(s => s.passes).length;
    const total = this.stories.length;
    return {
      completed,
      total,
      percentage: total > 0 ? Math.round((completed / total) * 100) : 0,
    };
  }

  private async gatherContext(story: Story): Promise<void> {
    this.tui.log(`Gathering context for: ${story.title}`);
    
    const context = await this.contextGatherer.gather({
      story,
      stories: this.stories,
      agentsMdPath: this.config.agentsMdFile,
    });
    
    this.tui.setContext(context);
  }

  private async executeStory(story: Story): Promise<AgentResult> {
    this.tui.log(`Executing: ${story.title}`);
    this.tui.setStatus('executing');
    
    const prompt = this.buildPrompt(story);
    const result = await this.agent.execute(prompt);
    
    this.tui.setStatus('idle');
    return result;
  }

  private buildPrompt(story: Story): string {
    return `# Task: ${story.title}

**ID:** ${story.id}
**Phase:** ${story.phase}
**Priority:** ${story.priority}

## Description
${story.description}

## Acceptance Criteria
${story.acceptanceCriteria.map(c => `- [ ] ${c}`).join('\n')}

## Instructions

### Ultrawork Pattern
You are running in Ralph-driven development mode with OhMyOpenCode. Leverage the full power of multi-agent orchestration:

1. **Delegate research** - Use \`librarian\` agent for documentation and external APIs
2. **Explore codebase** - Use \`explore\` agent for finding patterns in this repo
3. **Frontend work** - Delegate to \`frontend-ui-ux-engineer\` agent
4. **Architecture decisions** - Consult \`oracle\` agent for complex decisions

### Execution Steps
1. Read current context from CLAI session
2. Implement the task following acceptance criteria
3. Run quality checks:
   - Typecheck: \`go build ./...\`
   - Tests: \`go test ./...\`
4. If all checks pass, commit with message: \`feat: ${story.id} - ${story.title}\`
5. Update \`.clai/stories.json\` to set \`"${story.id}".passes = true\`

### Critical Rules
- Complete ONE story per iteration
- Always run quality checks before committing
- Update \`AGENTS.md\` with discovered patterns
- Never push to remote, only commit
- Output \`<COMPLETE>\` ONLY when ALL stories have \`passes: true\`

## Quality Standards
- Type errors: NONE allowed
- Test failures: Must fix, not skip
- Lint errors: Must resolve

Go forth and code!
`;
  }

  private async commitStory(story: Story, result: AgentResult): Promise<void> {
    this.tui.log(`Committing: ${story.id}`);
    
    // The agent handles the commit, but we track it here
    this.tui.addCommit({
      id: story.id,
      title: story.title,
      files: result.filesChanged || [],
      timestamp: new Date(),
    });
  }

  private async learnPatterns(result: AgentResult): Promise<void> {
    if (result.patterns && result.patterns.length > 0) {
      for (const pattern of result.patterns) {
        await this.patternLearner.learn(pattern);
        this.tui.log(`Learned: ${pattern.name}`);
      }
    }
  }

  private updateStoryStatus(storyId: string, passed: boolean): void {
    const story = this.stories.find(s => s.id === storyId);
    if (story) {
      story.passes = passed;
      story.updated = new Date().toISOString();
    }
  }

  private async handleFailure(story: Story, qualityResult: ReturnType<typeof this.qualityGates.run>): Promise<void> {
    this.tui.log(`Failed: ${story.title}`, 'error');
    this.tui.addFailure({
      storyId: story.id,
      reason: qualityResult.failures.join(', '),
      timestamp: new Date(),
    });
    
    // Log to progress file
    const progressEntry = {
      iteration: this.currentIteration,
      storyId: story.id,
      failures: qualityResult.failures,
      timestamp: new Date().toISOString(),
    };
    
    // Would append to progress file here
  }

  private areAllComplete(): boolean {
    return this.stories.every(s => s.passes);
  }

  private async summarize(): Promise<IterationResult> {
    const completed = this.stories.filter(s => s.passes).length;
    const duration = Date.now() - this.startTime;
    
    await this.tui.showSummary({
      success: completed === this.stories.length,
      iterations: this.currentIteration,
      completedStories: completed,
      totalStories: this.stories.length,
      duration,
    });
    
    return {
      success: completed === this.stories.length,
      iterations: this.currentIteration,
      completedStories: completed,
      totalStories: this.stories.length,
      duration,
    };
  }

  pause(): void {
    this.isPaused = true;
  }

  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
}
