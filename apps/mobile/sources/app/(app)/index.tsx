import { Text, View, Pressable } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import * as React from 'react';
import { router, useRouter } from "expo-router";
import { Redirect } from 'expo-router';
import { useProductAuth } from '@/weknora/auth/session';
import { WorkbenchScreen } from '@/weknora/workbench/WorkbenchScreen';

export default function Home() {
    const auth = useProductAuth();
    if (auth.loading) return null;
    if (!auth.credential) return <Redirect href="/login" />;
    return <Authenticated />;
}

function Authenticated() {
    return (
        <View style={{ flex: 1 }}>
            <WorkbenchScreen />
            <ProductNavigation />
        </View>
    );
}

const PRODUCT_NAVIGATION = [
    { key: 'workbench', label: '工作台', route: '/' },
    { key: 'agent', label: 'Agent', route: '/new' },
    { key: 'resources', label: '资源', route: '/knowledge' },
    { key: 'space', label: '空间', route: '/settings/account' },
    { key: 'settings', label: '设置', route: '/settings' },
] as const;

/** Product navigation owns destinations; Happy remains the session renderer. */
function ProductNavigation() {
    const router = useRouter();
    const insets = useSafeAreaInsets();
    return (
        <View accessibilityLabel="product-navigation" style={{
            position: 'absolute', left: 0, right: 0, bottom: 0,
            paddingBottom: Math.max(insets.bottom, 8), paddingTop: 8,
            flexDirection: 'row', justifyContent: 'space-around',
            backgroundColor: 'rgba(20,20,24,0.92)',
        }}>
            {PRODUCT_NAVIGATION.map((item) => (
                <Pressable
                    key={item.key}
                    accessibilityRole="button"
                    accessibilityLabel={item.label}
                    onPress={() => router.push(item.route as never)}
                    style={{ minWidth: 56, alignItems: 'center', paddingHorizontal: 6, paddingVertical: 4 }}
                >
                    <Text style={{ color: '#fff', fontSize: 12 }}>{item.label}</Text>
                </Pressable>
            ))}
        </View>
    );
}
