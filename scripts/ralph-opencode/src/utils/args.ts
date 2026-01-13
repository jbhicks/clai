/**
 * CLI argument parser
 */

export function parseArgs(args: string[]): Record<string, any> {
  const result: Record<string, any> = {};
  let i = 0;

  while (i < args.length) {
    const arg = args[i];
    
    if (arg.startsWith('--')) {
      const key = arg.slice(2);
      
      if (args[i + 1] && !args[i + 1].startsWith('--')) {
        result[key] = args[i + 1];
        i += 2;
      } else {
        result[key] = true;
        i++;
      }
    } else if (arg.startsWith('-')) {
      const key = arg.slice(1);
      
      if (args[i + 1] && !args[i + 1].startsWith('--')) {
        result[key] = args[i + 1];
        i += 2;
      } else {
        result[key] = true;
        i++;
      }
    } else {
      // Positional argument
      if (!result._) {
        result._ = [];
      }
      result._.push(arg);
      i++;
    }
  }

  return result;
}
