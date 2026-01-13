/**
 * Logger - Pretty console logging
 */

const levels = {
  debug: 0,
  info: 1,
  warn: 2,
  error: 3,
} as const;

let currentLevel = levels.info;

export function setLevel(level: keyof typeof levels): void {
  currentLevel = levels[level];
}

export const logger = {
  debug: (message: string, ...args: any[]): void => {
    if (currentLevel <= levels.debug) {
      console.log(`\x1b[90m[DEBUG]\x1b[0m ${message}`, ...args);
    }
  },
  
  info: (message: string, ...args: any[]): void => {
    if (currentLevel <= levels.info) {
      console.log(`\x1b[36m[INFO]\x1b[0m ${message}`, ...args);
    }
  },
  
  warn: (message: string, ...args: any[]): void => {
    if (currentLevel <= levels.warn) {
      console.log(`\x1b[33m[WARN]\x1b[0m ${message}`, ...args);
    }
  },
  
  error: (message: string, ...args: any[]): void => {
    if (currentLevel <= levels.error) {
      console.log(`\x1b[31m[ERROR]\x1b[0m ${message}`, ...args);
    }
  },
};
