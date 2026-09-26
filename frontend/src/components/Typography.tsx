import {
  StyleSheet,
  Text as NativeText,
  TextInput as NativeTextInput,
  type TextInputProps,
  type TextProps,
  type TextStyle,
} from 'react-native'

const fontFamilies = {
  400: 'GolosText_400Regular',
  500: 'GolosText_500Medium',
  600: 'GolosText_600SemiBold',
  700: 'GolosText_700Bold',
  800: 'GolosText_800ExtraBold',
  900: 'GolosText_900Black',
} as const

function resolveFontFamily(style: TextProps['style'] | TextInputProps['style']) {
  const weight = StyleSheet.flatten(style)?.fontWeight as TextStyle['fontWeight']
  const numericWeight = weight === 'bold' ? 700 : Number.parseInt(String(weight ?? 400), 10)

  if (numericWeight >= 900) return fontFamilies[900]
  if (numericWeight >= 800) return fontFamilies[800]
  if (numericWeight >= 700) return fontFamilies[700]
  if (numericWeight >= 600) return fontFamilies[600]
  if (numericWeight >= 500) return fontFamilies[500]
  return fontFamilies[400]
}

export function Text({ style, ...props }: TextProps) {
  return <NativeText {...props} style={[style, { fontFamily: resolveFontFamily(style), fontWeight: 'normal' }]} />
}

export function TextInput({ style, ...props }: TextInputProps) {
  return <NativeTextInput {...props} style={[style, { fontFamily: resolveFontFamily(style), fontWeight: 'normal' }]} />
}
