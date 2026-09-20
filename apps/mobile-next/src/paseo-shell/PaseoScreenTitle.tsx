import { useMemo, type ReactNode } from "react";
import { StyleSheet, Text, type StyleProp, type TextStyle } from "react-native";
import { generatedTokens } from "@/theme/tokens.generated";

export interface PaseoScreenTitleProps {
  children: ReactNode;
  numberOfLines?: number;
  testID?: string;
  style?: StyleProp<TextStyle>;
}

/**
 * Adapted from Paseo's canonical screen title. Leading controls are siblings,
 * never children, so the title continues to own its truncation behavior.
 */
export function PaseoScreenTitle({ children, numberOfLines = 1, testID, style }: PaseoScreenTitleProps) {
  const combinedStyle = useMemo(() => [styles.text, style], [style]);

  return (
    <Text style={combinedStyle} numberOfLines={numberOfLines} testID={testID}>
      {children}
    </Text>
  );
}

const styles = StyleSheet.create({
  text: {
    flexShrink: 1,
    minWidth: 0,
    fontSize: generatedTokens.typography.body.fontSize,
    fontWeight: "400",
    color: generatedTokens.colors.light.ink,
  },
});
