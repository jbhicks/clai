/**
 * OpenCode Agent - Multi-model orchestration via OhMyOpenCode
 */

import type { AgentResult } from './types.js';
import { logger } from '../utils/logger.js';
import { $ } from 'zx';
import { Readable } from 'stream';

export interface OpenCodeAgentConfig {
  model?: string;
  timeout?: number;
}

export class OpenCodeAgent {
  private config: Required<OpenCodeAgentConfig>;
  private socketPath = '/tmp/opencode.sock';

  constructor(config: OpenCodeAgentConfig = {}) {
    this.config = {
      model: config.model || 'opencode/claude-opus-4-5',
      timeout: config.timeout || 300000, // 5 minutes
    };
  }

  async execute(prompt: string): Promise<AgentResult> {
    const startTime = Date.now();
    
    try {
      logger.debug(`Executing with model: ${this.config.model}`);
      
      // Check if OpenCode MCP socket is available
      const socketAvailable = await this.checkSocket();
      
      if (socketAvailable) {
        return await this.executeViaSocket(prompt, startTime);
      } else {
        return await this.executeViaCLI(prompt, startTime);
      }
      
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : 'Unknown error',
        duration: Date.now() - startTime,
      };
    }
  }

  private async checkSocket(): Promise<boolean> {
    try {
      await $`test -S ${this.socketPath}`;
      return true;
    } catch {
      return false;
    }
  }

  private async executeViaSocket(prompt: string, startTime: number): Promise<AgentResult> {
    // MCP socket communication would go here
    // This is the "fancy" way - direct socket communication
    
    logger.info('Executing via MCP socket...');
    
    // Simulated socket response
    const response = await this.sendViaSocket({
      model: this.config.model,
      prompt,
      timeout: this.config.timeout,
    });

    return {
      success: true,
      output: response.output,
      filesChanged: response.filesChanged,
      patterns: response.patterns,
      duration: Date.now() - startTime,
    };
  }

  private async executeViaCLI(prompt: string, startTime: number): Promise<AgentResult> {
    // Fallback to CLI execution
    logger.info('Executing via CLI...');
    
    // Write prompt to temp file
    const promptFile = `/tmp/ralph-prompt-${Date.now()}.md`;
    await $`echo ${prompt} > ${promptFile}`;
    
    // Execute with OpenCode
    const result = await $`opencode --agent sisyphus --prompt ${promptFile} --timeout ${this.config.timeout / 1000}s`
      .timeout(this.config.timeout)
      .quiet()
      .nothrow();
    
    // Cleanup
    await $`rm -f ${promptFile}`;

    const output = result.stdout || result.stderr;
    
    // Parse output for completion signal
    const isComplete = output.includes('<COMPLETE>');
    const hasError = output.toLowerCase().includes('error') && !output.includes('error handling');
    
    return {
      success: !hasError,
      output,
      duration: Date.now() - startTime,
    };
  }

  private async sendViaSocket(request: {
    model: string;
    prompt: string;
    timeout: number;
  }): Promise<{ output: string; filesChanged: string[]; patterns: any[] }> {
    // This would be a real MCP socket implementation
    // For now, return a simulated response
    
    // In production, this would use net.Socket to communicate with OpenCode
    const net = await import('net');
    
    return new Promise((resolve, reject) => {
      const socket = new net.Socket();
      
      const timeout = setTimeout(() => {
        socket.destroy();
        reject(new Error('Socket timeout'));
      }, request.timeout);
      
      socket.connect(this.socketPath, () => {
        socket.write(JSON.stringify(request));
      });
      
      let data = '';
      
      socket.on('data', (chunk) => {
        data += chunk.toString();
      });
      
      socket.on('end', () => {
        clearTimeout(timeout);
        socket.destroy();
        
        try {
          const response = JSON.parse(data);
          resolve(response);
        } catch {
          resolve({ output: data, filesChanged: [], patterns: [] });
        }
      });
      
      socket.on('error', (error) => {
        clearTimeout(timeout);
        socket.destroy();
        reject(error);
      });
    });
  }

  setModel(model: string): void {
    this.config.model = model;
  }
}
