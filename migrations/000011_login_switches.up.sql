-- 登录页交互开关：图形验证码 / 滑块校验
-- 背景：验证码开关此前只能在配置文件里改（需重启），滑块校验则写死在前端。
-- 现在统一收敛到 sys_config（is_public=1），后台「参数配置」即可热切换，登录页读取公开参数即时生效。
INSERT IGNORE INTO sys_config (id, name, config_key, value, value_type, is_builtin, is_public, tenant_id, create_time, remark) VALUES
(4, '是否开启图形验证码', 'sys.login.captcha', 'true', 'bool', 1, 1, 0, NOW(), '关闭后登录页不再显示验证码，后端也不再校验（配置文件 auth.captcha_enabled 为总闸）'),
(7, '是否开启滑块校验', 'sys.login.slider', 'true', 'bool', 1, 1, 0, NOW(), '登录页的拖拽滑块人机校验，纯前端交互，关闭后直接点击登录即可');
