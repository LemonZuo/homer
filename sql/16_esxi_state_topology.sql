-- 给 esxi_state 加 topology_json 列：
--   存 { vswitches:[{name,uplinks,portgroups}], vm_nics:[{vmid,vm_name,vswitch,portgroup,mac,ip,team_uplink}] }
--   前端按 pNIC → vSwitch → Portgroup → VMs 四列渲染网络拓扑。
--
-- 幂等：先看列是否存在，缺了才 ADD。

DROP PROCEDURE IF EXISTS homer_add_esxi_state_topology;
DELIMITER //
CREATE PROCEDURE homer_add_esxi_state_topology()
BEGIN
  IF EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.TABLES
             WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'esxi_state') THEN
    IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                   WHERE TABLE_SCHEMA = DATABASE()
                     AND TABLE_NAME = 'esxi_state'
                     AND COLUMN_NAME = 'topology_json') THEN
      ALTER TABLE `esxi_state`
        ADD COLUMN `topology_json` TEXT NULL
          COMMENT '网络拓扑'
          AFTER `nic_json`;
    END IF;
  END IF;
END //
DELIMITER ;
CALL homer_add_esxi_state_topology();
DROP PROCEDURE IF EXISTS homer_add_esxi_state_topology;
