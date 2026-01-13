# Border and Color Rendering Fixes

## Problem Identified
User reported that execution results (tool messages) were "fucking up borders" and colors still didn't look "quite like native."

## Root Causes Found

### 1. Tool Message Border Issues
**Problem**: Tool messages were using `MarginLeft(2)` instead of proper borders, causing:
- Inconsistent appearance with user/assistant messages
- Visual misalignment 
- "Broken" border appearance

**Solution**: Added proper rounded borders to ToolMessage style:
```go
ToolMessage: lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color(ui.Theme.Bright.Blue)).
    Background(lipgloss.Color(ui.Theme.Dim.Black)).
    Foreground(lipgloss.Color(ui.Theme.Dim.White)).
    Italic(true).
    Padding(0, 1),
```

### 2. Width Calculation Issues
**Problem**: Tool messages weren't using `padLinesToWidth()` function like user messages, causing:
- Background color gaps
- Inconsistent line widths
- Visual artifacts

**Solution**: Applied proper width padding:
```go
bubble := themeStyles.ToolMessage.Render(toolHeader + "\n" + wrappedContent)
bubble = padLinesToWidth(bubble, c.Width, lipgloss.Color(c.Theme.Theme.Primary.Background))
rendered = bubble
```

### 3. Color Profile Issues
**Problem**: lipgloss was auto-detecting color profile, potentially using suboptimal conversion:
- Inconsistent color rendering
- Poor color quality in 256-color terminals

**Solution**: Explicit color profile setting:
```go
if HasTrueColor() {
    lipgloss.SetColorProfile(termenv.TrueColor)
} else {
    lipgloss.SetColorProfile(termenv.ANSI256)
}
```

### 4. Enhanced 256-Color Palette
**Problem**: Basic 256-color codes weren't vibrant enough:
- Background 17 → 235 (richer dark)
- Red 196 → 197 (brighter)  
- Green 46 → 48 (more vibrant)
- Blue 141 → 99 (deeper purple-blue)
- Background improvements for both themes

## Files Modified

### `internal/ui/styles.go`
- ✅ Added termenv import
- ✅ Enhanced DraculaTheme256 with better colors
- ✅ Enhanced TokyoNightTheme256 with richer backgrounds
- ✅ Added explicit color profile setting
- ✅ Fixed ToolMessage style with proper borders

### `internal/ui/chat.go`
- ✅ Fixed tool message rendering with proper width padding
- ✅ Removed problematic wrapper logic
- ✅ Applied consistent `padLinesToWidth` usage

## Expected Results

### Before Fix:
- ❌ Tool messages had no borders (just margin)
- ❌ Background color gaps in tool output
- ❌ Inconsistent message alignment
- ❌ Suboptimal color conversion
- ❌ Basic 256-color palette

### After Fix:
- ✅ Tool messages have proper rounded borders
- ✅ Consistent background colors across all message types
- ✅ Proper width alignment and padding
- ✅ Explicit color profile control
- ✅ Enhanced 256-color palette
- ✅ Native-like appearance

## Testing

### Visual Test:
```bash
./test_color_rendering.sh
```

### App Test:
```bash
make dev
# Execute some code to see tool messages
# Check that borders render properly
# Verify colors look vibrant
```

### Color Profile Test:
```bash
# Normal 256-color mode
make dev

# Force true color (if terminal supports)
CLAI_FORCE_TRUE_COLOR=1 make dev
```

## Debug Information

Check `debug.log` for:
- `[COLOR-DETECT]` messages showing color detection
- Theme selection decisions
- Color profile being set

## Remaining Issues

If colors still don't look "native":
1. **Terminal Settings**: Check for true color support in terminal config
2. **Font Rendering**: Some fonts render colors differently
3. **WezTerm Remote**: May need specific remote session configuration
4. **SSH Color Forwarding**: Ensure SSH preserves color capabilities

The border issues should now be completely resolved, and color quality significantly improved.