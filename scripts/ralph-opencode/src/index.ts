#!/usr/bin/env node
/**
 * Ralph-OpenCode - Fancy Autonomous Development Loop
 * 
 * Multi-agent orchestration using OhMyOpenCode with beautiful TUI
 * 
 * @author CLAI Team
 * @license MIT
 */

import { RalphLoop } from './orchestrator.js';
import { parseArgs } from './utils/args.js';
import { loadConfig } from './config/index.js';
import { logger } from './utils/logger.js';
import { banner } from './utils/banner.js';

async function main() {
  // Display beautiful banner
  banner.print();
  
  // Parse command line arguments
  const args = parseArgs(process.argv.slice(2));
  
  // Load configuration
  const config = await loadConfig(args.config);
  
  // Create and run Ralph loop
  const loop = new RalphLoop({
    ...config,
    ...args,
    verbose: args.verbose ?? config.verbose ?? false,
    maxIterations: args.maxIterations ?? config.maxIterations ?? 50,
  });
  
  // Handle signals gracefully
  process.on('SIGINT', () => {
    logger.info('Received SIGINT, shutting down gracefully...');
    loop.pause();
    process.exit(0);
  });
  
  process.on('SIGTERM', () => {
    logger.info('Received SIGTERM, shutting down gracefully...');
    loop.pause();
    process.exit(0);
  });
  
  // Run the loop
  const result = await loop.run();
  
  // Exit with appropriate code
  process.exit(result.success ? 0 : 1);
}

main().catch((error) => {
  logger.error('Fatal error:', error);
  process.exit(1);
});
