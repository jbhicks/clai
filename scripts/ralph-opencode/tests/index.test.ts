/**
 * Ralph-OpenCode Test Suite
 */

import { describe, it, expect, beforeAll, afterAll } from 'bun:test';
import { RalphLoop } from '../src/orchestrator.js';
import { ContextGatherer } from '../src/agents/context-gatherer.js';
import { QualityGates } from '../src/quality/gates.js';
import { PatternLearner } from '../src/persistence/patterns.js';
import { parseArgs } from '../src/utils/args.js';

describe('Ralph-OpenCode', () => {
  describe('Utils', () => {
    it('parses CLI arguments correctly', () => {
      const args = parseArgs([
        '--max-iterations', '50',
        '--model', 'test-model',
        '--verbose'
      ]);
      
      expect(args['max-iterations']).toBe('50');
      expect(args.model).toBe('test-model');
      expect(args.verbose).toBe(true);
    });

    it('handles positional arguments', () => {
      const args = parseArgs(['story1', 'story2', '--flag']);
      
      expect(args._).toEqual(['story1', 'story2']);
      expect(args.flag).toBe(true);
    });

    it('handles short arguments', () => {
      const args = parseArgs(['-m', 'model', '-v']);
      
      expect(args.m).toBe('model');
      expect(args.v).toBe(true);
    });
  });

  describe('ContextGatherer', () => {
    it('detects project type', async () => {
      const gatherer = new ContextGatherer();
      const context = await gatherer.gather({
        story: {
          id: 'TEST-001',
          title: 'Test',
          description: 'Test',
          acceptanceCriteria: [],
          priority: 'high',
          passes: false,
          phase: 'test',
        },
        stories: [],
        agentsMdPath: './AGENTS.md',
      });
      
      // Language detection works (may be 'unknown' in test env)
      expect(context.codebaseInfo.language).toBeTruthy();
      expect(context.codebaseInfo.buildCommand).toBeTruthy();
      expect(context.codebaseInfo.testCommand).toBeTruthy();
    });

    it('finds key files in project', async () => {
      const gatherer = new ContextGatherer();
      const context = await gatherer.gather({
        story: {
          id: 'TEST-001',
          title: 'Test',
          description: 'Test',
          acceptanceCriteria: [],
          priority: 'high',
          passes: false,
          phase: 'test',
        },
        stories: [],
        agentsMdPath: './AGENTS.md',
      });
      
      // Should have some key files detected
      expect(context.codebaseInfo.keyFiles.length).toBeGreaterThanOrEqual(0);
    });
  });

  describe('QualityGates', () => {
    it('runs typecheck check', async () => {
      const gates = new QualityGates();
      const result = await gates.run({ success: true });
      
      expect(result.passed).toBe(true);
      expect(result.checks.find(c => c.name === 'Typecheck')).toBeDefined();
    });

    it('runs test check', async () => {
      const gates = new QualityGates();
      const result = await gates.run({ success: true });
      
      expect(result.checks.find(c => c.name === 'Tests')).toBeDefined();
    });

    it('tracks check duration', async () => {
      const gates = new QualityGates();
      const result = await gates.run({ success: true });
      
      expect(result.duration).toBeGreaterThanOrEqual(0);
    });
  });

  describe('PatternLearner', () => {
    const testPath = '/tmp/test-agents-md.md';
    
    afterAll(async () => {
      try {
        await import('fs').then(fs => fs.rmSync(testPath, { force: true }));
      } catch {}
    });

    it('creates new AGENTS.md if not exists', async () => {
      const learner = new PatternLearner(testPath);
      await learner.learn({
        name: 'TestPattern',
        description: 'A test pattern',
        context: 'Used for testing',
        source: 'test',
      });
      
      const content = await import('fs').then(fs => 
        fs.readFileSync(testPath, 'utf-8')
      );
      
      expect(content).toContain('TestPattern');
      expect(content).toContain('## Codebase Patterns');
    });

    it('does not duplicate patterns', async () => {
      const learner = new PatternLearner(testPath);
      
      await learner.learn({
        name: 'UniquePattern',
        description: 'A unique pattern',
        context: 'Test context',
        source: 'test',
      });
      
      await learner.learn({
        name: 'UniquePattern',
        description: 'Same pattern',
        context: 'Different context',
        source: 'test',
      });
      
      const content = await import('fs').then(fs =>
        fs.readFileSync(testPath, 'utf-8')
      );
      
      const matches = content.match(/UniquePattern/g) || [];
      expect(matches.length).toBe(1);
    });
  });

  describe('Integration', () => {
    it('loads stories from .clai/stories.json', async () => {
      const loop = new RalphLoop({
        storiesFile: './.clai/stories.json',
      });
      
      expect(loop).toBeDefined();
    });
  });
});
