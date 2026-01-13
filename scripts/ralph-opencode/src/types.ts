/**
 * Types for Ralph-OpenCode
 */

/**
 * Ralph loop configuration
 */
export interface LoopConfig {
  /** Path to stories JSON file */
  storiesFile?: string;
  /** Path to progress JSON file */
  progressFile?: string;
  /** Path to AGENTS.md file */
  agentsMdFile?: string;
  /** Maximum iterations before stopping */
  maxIterations?: number;
  /** Verbose logging */
  verbose?: boolean;
  /** Model to use */
  model?: string;
  /** Branch name for commits */
  branchName?: string;
  /** Enable auto-handoff for large tasks */
  autoHandoff?: boolean;
}

/**
 * User story from prd.json/stories.json
 */
export interface Story {
  id: string;
  title: string;
  description: string;
  acceptanceCriteria: string[];
  priority: 'high' | 'medium' | 'low' | 'P0' | 'P1' | 'P2' | 'P3';
  passes: boolean;
  phase: string;
  notes?: string;
  created?: string;
  updated?: string;
}

/**
 * Result from agent execution
 */
export interface AgentResult {
  success: boolean;
  filesChanged?: string[];
  patterns?: Pattern[];
  output?: string;
  error?: string;
  duration?: number;
}

/**
 * Code pattern learned during execution
 */
export interface Pattern {
  name: string;
  description: string;
  context: string;
  source: string;
}

/**
 * Quality gate result
 */
export interface QualityResult {
  passed: boolean;
  checks: QualityCheck[];
  failures: string[];
  duration: number;
}

/**
 * Individual quality check
 */
export interface QualityCheck {
  name: string;
  passed: boolean;
  output?: string;
  error?: string;
  duration: number;
}

/**
 * Commit information
 */
export interface CommitInfo {
  id: string;
  title: string;
  files: string[];
  timestamp: Date;
}

/**
 * Failure information
 */
export interface FailureInfo {
  storyId: string;
  reason: string;
  timestamp: Date;
}

/**
 * Context gathered before task execution
 */
export interface GatheredContext {
  story: Story;
  patterns: Pattern[];
  codebaseInfo: CodebaseInfo;
  relevantAgentsMd?: string;
}

/**
 * Codebase information
 */
export interface CodebaseInfo {
  language: string;
  buildCommand: string;
  testCommand: string;
  keyFiles: string[];
  patterns: string[];
}

/**
 * Iteration result summary
 */
export interface IterationResult {
  success: boolean;
  iterations: number;
  completedStories: number;
  totalStories: number;
  duration?: number;
  error?: string;
}

/**
 * TUI initialization options
 */
export interface TUIOptions {
  totalStories: number;
  branchName: string;
}

/**
 * Summary display data
 */
export interface SummaryData {
  success: boolean;
  iterations: number;
  completedStories: number;
  totalStories: number;
  duration: number;
}
